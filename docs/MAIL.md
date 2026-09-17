# Sending mail from a site that blocks SMTP

Most corporate networks close outbound mail ports. It is an ordinary
anti-spam posture and it does not get reopened quickly. On the EMSE
installation, measured rather than assumed:

| Destination | Port | Result |
|---|---|---|
| the institution's own relay | 25 | closed |
| Google | 587 | closed |
| Microsoft | 587 | closed |
| the domain's own provider | 465, 587 | closed |
| anything over HTTPS | 443 | open |

## Why a transport in the platform was not the answer

The obvious fix is to give Noryx an HTTP mail transport. It would have
delivered alerts and notifications, and **not** the thing that was actually
needed.

Invitations and password resets do not come from the platform. They come from
Keycloak, which speaks SMTP and nothing else, and which is not ours to modify.
An HTTP transport in Noryx would have solved everything except onboarding.

## The shape

```
Keycloak ─┐
          ├─SMTP─▶ noryx-mail (bridge) ─HTTPS─▶ noryx-mail (relay) ─SMTP auth─▶ provider
Noryx ────┘        in the cluster                on a public host
```

Two modes of one binary, chosen by `NORYX_MAIL_MODE`. Keycloak and the platform
point at the bridge as their SMTP server; neither was modified, and neither
knows about the rest.

**The bridge** listens for SMTP inside the cluster, so nothing has to leave it,
and posts each envelope to the relay over HTTPS, which does.

**The relay** submits to the provider that holds the domain. It does not
deliver mail itself, on purpose: the domain's SPF record ends in `-all`, so
only that provider may send for it. A relay that delivered directly would fail
authentication and land in spam, which is worse than not arriving at all.

## What stops it being an open relay

A token leaks eventually. What must remain true afterwards is that it cannot
be used to send as anybody:

- **`NORYX_MAIL_ALLOWED_SENDERS` is mandatory.** The relay refuses to start
  without it rather than defaulting to permissive. An exact address
  (`noreply@example.org`) or a whole domain (`@example.org`).
- **An hourly ceiling**, on a sliding window. An hourly counter reset on the
  hour lets twice the quota through across the boundary.
- **The token is compared in constant time**, because one compared byte by
  byte is one that can be guessed byte by byte.
- **Nothing is queued.** A refusal is a 4xx the sender knows how to retry;
  there is no spool to fill and no message held somewhere nobody looks.

## Configuration

Bridge, in the cluster:

    NORYX_MAIL_MODE=bridge
    NORYX_MAIL_LISTEN_ADDR=:1025
    NORYX_MAIL_RELAY_URL=https://mail.example.org
    NORYX_MAIL_TOKEN=<shared with the relay>

Relay, on the public host:

    NORYX_MAIL_MODE=relay
    NORYX_MAIL_LISTEN_ADDR=127.0.0.1:8025
    NORYX_MAIL_TOKEN=<shared with the bridge>
    NORYX_MAIL_SMTP_HOST=ssl0.example.net:587
    NORYX_MAIL_SMTP_USERNAME=<mailbox>
    NORYX_MAIL_SMTP_PASSWORD=<mailbox password>
    NORYX_MAIL_ALLOWED_SENDERS=noreply@example.org,@sub.example.org
    NORYX_MAIL_HOURLY_LIMIT=200

Then point Keycloak's realm at `noryx-mail-bridge.noryx.svc.cluster.local:1025`,
no authentication, no TLS — the hop does not leave the cluster, and encryption
starts at the one that does.

## What this is not

Not a mail server. No queue, no retry, no local delivery, no DKIM signing —
the provider does that. It accepts an envelope or it refuses it, and the
message body travels untouched: Keycloak composes its own MIME, and a relay
that rebuilt it would break accented text on the day nobody is looking.

## The alternative, if the network team will have it

A single firewall rule — `<cluster> → <provider submission host>:587` — removes
both components. That is a narrow, ordinary request, quite unlike "open
outbound SMTP", and worth asking for in parallel. It was not asked for here
because waiting on it blocks the onboarding it would serve.

## What is deployed today

Both installations send through the same relay. Recorded here because the
pieces sit on three machines and none of them is obvious from the others.

| Piece | Where | What it is |
|---|---|---|
| Relay | the public web host | systemd unit `noryx-mail`, config `/etc/noryx-mail.env` (mode 600, root), binary `/opt/noryx-mail` |
| Published at | nginx on the same host | `https://<site>/noryx-mail/` → `127.0.0.1:8025` |
| Bridge | each cluster, namespace `noryx` | Deployment + Service `noryx-mail-bridge`, port 1025, secret `noryx-mail-bridge` |
| Upstream | the domain's mailbox provider | submission on port 587, STARTTLS, authenticating as the sending mailbox |

The bridge runs from the backend image with a different command, so it follows
the backend's version. It is built into **both** Dockerfiles — the Community one
and the Enterprise one — because the Enterprise edition is built from its own
file and adding the binary to one produces an image without it in the other.

### Who sends

Keycloak and the platform both point at the bridge as their SMTP server, and
both read the **same** configuration: the platform's SMTP screen writes into
the Keycloak realm, so there is one source of truth and configuring the realm
configures both.

Realm settings: host `noryx-mail-bridge.noryx.svc.cluster.local`, port `1025`,
no TLS, no authentication — the hop does not leave the cluster, and encryption
starts at the one that does.

### The trap when reading it back

`kcadm.sh get realms/<realm> --fields smtpServer` prints `{}` for a nested
object **even when the value is set**. Reading it back that way says the write
failed when it did not. Use the admin REST API to verify, or the platform's own
SMTP screen.

### Sending limits

Two ceilings, and the lower one should be ours so that a refusal comes with a
message we wrote:

- the mailbox provider's own daily quota;
- `NORYX_MAIL_HOURLY_LIMIT` on the relay, set below it.

Past the relay's limit the bridge returns a temporary 4xx and the sender
retries. Past the provider's, the failure arrives as an SMTP error in a log
nobody reads, halfway through a batch of invitations.

### Before inviting anybody

- the account needs an email address, or Keycloak composes nothing at all;
- the required action the invitation relies on (`UPDATE_PASSWORD`) must be
  enabled on the realm, or the mail is sent and the action is silently ignored.

### Where to look when a message does not arrive

Follow the chain in order; each hop names the one before it.

    in the cluster:  kubectl -n noryx logs deploy/noryx-mail-bridge   → "accepte de=… vers=N"
    on the relay:    journalctl -u noryx-mail                         → "remis de=… vers=N"

Bridge silent: the sender composed nothing — missing address, or the required
action never fired. Bridge accepted and relay silent: the HTTPS hop. Both
logged: the message reached the provider and the rest is theirs.

The bridge logs `connection refused` while the relay is unreachable and repairs
itself; those lines are not an incident on their own.
