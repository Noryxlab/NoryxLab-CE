package main

import (
	"crypto/subtle"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

// Le relai : HTTPS en entree, SMTP authentifie vers le fournisseur en sortie.
//
// Il vit sur la machine publique, celle qui porte deja le certificat. Il ne
// remet rien lui-meme : il soumet au fournisseur qui detient le domaine, parce
// que le SPF du domaine est en `-all` et n'autorise que lui. Un relai qui
// emettrait directement echouerait l'authentification et finirait en
// indesirable, ce qui est pire que de ne pas partir.
type relay struct {
	token    string
	host     string // hote:port de soumission
	username string
	password string
	// allowedSenders borne les adresses d'enveloppe acceptees. Sans cela, un
	// jeton qui fuit transforme ce service en relai ouvert, et un relai ouvert
	// devient une machine a indesirables avant la fin de la journee.
	allowedSenders []string

	limiter *rateLimiter
}

func (r *relay) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/send", r.send)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	return mux
}

func (r *relay) send(w http.ResponseWriter, request *http.Request) {
	if !r.authorised(request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var message envelope
	if err := json.NewDecoder(http.MaxBytesReader(w, request.Body, maxMessageBytes+1<<16)).Decode(&message); err != nil {
		http.Error(w, "invalid envelope", http.StatusBadRequest)
		return
	}
	if err := message.validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !r.senderAllowed(message.From) {
		// Refuse avant l'envoi, et dit pourquoi dans le journal : c'est la
		// difference entre un relai et un relai ouvert.
		log.Printf("expediteur refuse: %s", message.From)
		http.Error(w, "sender not allowed", http.StatusForbidden)
		return
	}
	if !r.limiter.allow() {
		// 429 plutot qu'un refus definitif : le pont retentera, et l'appelant
		// SMTP recevra un 4xx qui l'invite a reessayer.
		http.Error(w, "too many messages", http.StatusTooManyRequests)
		return
	}
	if err := r.submit(message); err != nil {
		log.Printf("soumission refusee par %s: %v", r.host, err)
		http.Error(w, "upstream refused", http.StatusBadGateway)
		return
	}
	log.Printf("remis de=%s vers=%d octets=%d", message.From, len(message.To), len(message.Data))
	w.WriteHeader(http.StatusAccepted)
}

func (r *relay) authorised(request *http.Request) bool {
	header := strings.TrimSpace(request.Header.Get("Authorization"))
	presented := strings.TrimPrefix(header, "Bearer ")
	// Comparaison a temps constant : un jeton se devine octet par octet quand
	// la comparaison s'arrete au premier ecart.
	return subtle.ConstantTimeCompare([]byte(presented), []byte(r.token)) == 1
}

func (r *relay) senderAllowed(from string) bool {
	if len(r.allowedSenders) == 0 {
		return false
	}
	address := strings.ToLower(addressOnly(from))
	for _, allowed := range r.allowedSenders {
		allowed = strings.ToLower(strings.TrimSpace(allowed))
		if allowed == "" {
			continue
		}
		// Un domaine entier s'ecrit "@domaine.fr".
		if strings.HasPrefix(allowed, "@") {
			if strings.HasSuffix(address, allowed) {
				return true
			}
			continue
		}
		if address == allowed {
			return true
		}
	}
	return false
}

func (r *relay) submit(message envelope) error {
	host, _, err := net.SplitHostPort(r.host)
	if err != nil {
		return fmt.Errorf("hote de soumission illisible %q: %w", r.host, err)
	}
	recipients := make([]string, 0, len(message.To))
	for _, recipient := range message.To {
		recipients = append(recipients, addressOnly(recipient))
	}
	auth := smtp.PlainAuth("", r.username, r.password, host)
	// TLS implicite sur 465, STARTTLS ailleurs : les deux existent chez les
	// fournisseurs et le port dit lequel.
	if strings.HasSuffix(r.host, ":465") {
		return submitImplicitTLS(r.host, host, auth, addressOnly(message.From), recipients, message.Data)
	}
	return smtp.SendMail(r.host, auth, addressOnly(message.From), recipients, message.Data)
}

func submitImplicitTLS(address, host string, auth smtp.Auth, from string, to []string, data []byte) error {
	conn, err := tls.Dial("tcp", address, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	if err != nil {
		return err
	}
	defer conn.Close()
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.Auth(auth); err != nil {
		return err
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(data); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}

// rateLimiter borne ce qu'une fuite de jeton peut couter.
//
// Une fenetre glissante plutot qu'un compteur horaire : un compteur remis a
// zero a l'heure ronde autorise deux fois le quota a cheval sur la bascule.
type rateLimiter struct {
	mu     sync.Mutex
	window time.Duration
	limit  int
	sent   []time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{limit: limit, window: window}
}

func (l *rateLimiter) allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-l.window)
	kept := l.sent[:0]
	for _, at := range l.sent {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	l.sent = kept
	if len(l.sent) >= l.limit {
		return false
	}
	l.sent = append(l.sent, time.Now())
	return true
}
