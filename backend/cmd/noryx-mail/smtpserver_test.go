package main

import (
	"bufio"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

type captured struct {
	messages []envelope
	fail     error
}

func (c *captured) deliver(message envelope) error {
	if c.fail != nil {
		return c.fail
	}
	c.messages = append(c.messages, message)
	return nil
}

// Joue une vraie conversation sur une socket locale.
//
// Pas net.Pipe : il est synchrone, donc la reponse multiligne d'EHLO bloque
// l'ecrivain tant que le lecteur n'a pas tout consomme, et le test se fige au
// lieu d'echouer. Une socket a un tampon, comme un vrai client.
//
// La conversation n'est pas un echange une-ligne-une-reponse : EHLO repond sur
// trois lignes, et le corps du message n'obtient de reponse qu'apres le point
// final. Le harnais doit donc suivre le protocole, pas l'alterner.
type conversation struct {
	t      *testing.T
	conn   net.Conn
	reader *bufio.Reader
}

func dial(t *testing.T, sink deliverer) *conversation {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &smtpServer{hostname: "test", deliver: sink}
	go func() { _ = srv.serve(listener) }()
	t.Cleanup(func() { _ = listener.Close() })

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	// Un test qui pend est un test qui ne dit rien : mieux vaut echouer vite.
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	talk := &conversation{t: t, conn: conn, reader: bufio.NewReader(conn)}
	talk.expect("220")
	return talk
}

// reply lit une reponse complete, y compris multiligne : "250-" continue,
// "250 " termine.
func (c *conversation) reply() string {
	c.t.Helper()
	var lines []string
	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			c.t.Fatalf("lecture: %v (deja lu: %v)", err, lines)
		}
		line = strings.TrimRight(line, "\r\n")
		lines = append(lines, line)
		if len(line) < 4 || line[3] != '-' {
			return strings.Join(lines, " | ")
		}
	}
}

func (c *conversation) send(line string) string {
	c.t.Helper()
	if _, err := c.conn.Write([]byte(line + "\r\n")); err != nil {
		c.t.Fatalf("ecriture %q: %v", line, err)
	}
	return c.reply()
}

// body envoie le corps puis le point final, et ne lit qu'une reponse : le
// serveur ne repond pas ligne a ligne pendant DATA.
func (c *conversation) body(lines ...string) string {
	c.t.Helper()
	for _, line := range lines {
		if _, err := c.conn.Write([]byte(line + "\r\n")); err != nil {
			c.t.Fatalf("ecriture du corps: %v", err)
		}
	}
	return c.send(".")
}

func (c *conversation) expect(prefix string) string {
	c.t.Helper()
	got := c.reply()
	if !strings.HasPrefix(got, prefix) {
		c.t.Fatalf("attendu %s, recu %q", prefix, got)
	}
	return got
}

func TestUnEchangeComplet(t *testing.T) {
	sink := &captured{}
	talk := dial(t, sink)
	talk.send("EHLO keycloak")
	talk.send("MAIL FROM:<noreply@noryxlab.ai>")
	talk.send("RCPT TO:<chercheur@inria.fr>")
	if got := talk.send("DATA"); !strings.HasPrefix(got, "354") {
		t.Fatalf("DATA refuse: %s", got)
	}
	if got := talk.body("Subject: Bienvenue", "", "Votre acces est pret."); !strings.HasPrefix(got, "250") {
		t.Fatalf("message refuse: %s", got)
	}
	talk.send("QUIT")

	if len(sink.messages) != 1 {
		t.Fatalf("attendu un message, recu %d", len(sink.messages))
	}
	message := sink.messages[0]
	if message.From != "noreply@noryxlab.ai" {
		t.Errorf("expediteur: %q", message.From)
	}
	if len(message.To) != 1 || message.To[0] != "chercheur@inria.fr" {
		t.Errorf("destinataires: %v", message.To)
	}
	if !strings.Contains(string(message.Data), "Votre acces est pret.") {
		t.Errorf("corps perdu: %q", message.Data)
	}
	// Les en-tetes de l'emetteur traversent intacts : Keycloak compose son
	// propre MIME, et un relai qui le reconstruirait casserait les accents.
	if !strings.Contains(string(message.Data), "Subject: Bienvenue") {
		t.Errorf("en-tetes perdus: %q", message.Data)
	}
}

func TestLePointDoubleEstDefait(t *testing.T) {
	// Le protocole prefixe d'un point toute ligne du corps qui en commence un.
	// Oublier de le defaire corrompt en silence tout message contenant une
	// telle ligne, et le defaut ne se voit que chez le destinataire.
	sink := &captured{}
	talk := dial(t, sink)
	talk.send("EHLO k")
	talk.send("MAIL FROM:<a@b.fr>")
	talk.send("RCPT TO:<c@d.fr>")
	talk.send("DATA")
	talk.body("Subject: code", "", "..point en debut de ligne", "normal")
	talk.send("QUIT")

	if len(sink.messages) != 1 {
		t.Fatal("message perdu")
	}
	body := string(sink.messages[0].Data)
	if !strings.Contains(body, "\r\n.point en debut de ligne\r\n") {
		t.Errorf("le point double n'a pas ete defait: %q", body)
	}
}

func TestDataRefuseAvantLeCorpsSansDestinataire(t *testing.T) {
	// Accepter DATA puis echouer laisserait l'appelant croire que le message
	// est parti. Le refus doit venir avant que le corps soit ecrit.
	sink := &captured{}
	talk := dial(t, sink)
	talk.send("EHLO k")
	if got := talk.send("DATA"); !strings.HasPrefix(got, "503") {
		t.Errorf("DATA sans enveloppe aurait du etre refuse, recu: %s", got)
	}
	talk.send("QUIT")
	if len(sink.messages) != 0 {
		t.Error("un message a ete remis sans destinataire")
	}
}

func TestUneRemiseImpossibleDonneUn4xx(t *testing.T) {
	// L'echec est le notre, pas celui de l'appelant : un client qui reessaie a
	// raison de le faire, et un 5xx lui dirait d'abandonner.
	sink := &captured{fail: errors.New("relai injoignable")}
	talk := dial(t, sink)
	talk.send("EHLO k")
	talk.send("MAIL FROM:<a@b.fr>")
	talk.send("RCPT TO:<c@d.fr>")
	talk.send("DATA")
	if got := talk.body("Subject: x", "", "corps"); !strings.HasPrefix(got, "451") {
		t.Errorf("attendu un 4xx temporaire, recu: %s", got)
	}
	talk.send("QUIT")
}

func TestLesFormesDAdresseQueLesClientsEnvoient(t *testing.T) {
	for _, form := range []struct{ raw, want string }{
		{"FROM:<a@b.fr>", "a@b.fr"},
		{"FROM: <a@b.fr>", "a@b.fr"},
		{"FROM:<a@b.fr> SIZE=1234", "a@b.fr"},
		{"FROM:a@b.fr", "a@b.fr"},
	} {
		if got := extractAddress(form.raw, "FROM:"); got != form.want {
			t.Errorf("%q -> %q, attendu %q", form.raw, got, form.want)
		}
	}
}
