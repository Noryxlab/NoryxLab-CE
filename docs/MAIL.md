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
