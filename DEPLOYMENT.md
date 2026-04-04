# YinMoNote Deployment Reference

Practical guide for operators. No fluff.

---

## Quick Deploy — Docker (Recommended)

```bash
make docker          # builds .tar image → dist/
make install-docker  # loads image + interactive setup (data dir, port, access mode)
```

`install-docker` creates a Docker Compose file and a systemd unit. Start with:

```bash
sudo systemctl start yinmonote
sudo systemctl enable yinmonote   # persist across reboots
```

---

## Native Binary

```bash
make build    # produces dist/yinmonote-<os>-<arch>
make install  # interactive: data dir, port, LaunchAgent (macOS) or systemd (Linux)
```

No runtime dependencies. The binary embeds the frontend.

---

## TLS Options

| Variable | Effect |
|---|---|
| `ACME_DOMAIN=notes.example.com` | Let's Encrypt auto-cert (requires port 443 reachable) |
| `TLS_CERT=<path> TLS_KEY=<path>` | Manual cert/key pair |
| `TLS_SELF=1` | Self-signed cert; CA downloadable at `/ca.crt` |
| (none set) | Plain HTTP — suitable behind a TLS-terminating reverse proxy |

---

## Production Deploy — vpc_tengxun

The project's production server uses passwordless SSH (key-based auth).

Resolve the SSH agent socket, then copy and restart:

```bash
export SSH_AUTH_SOCK=$(ls /tmp/ssh-*/agent.* 2>/dev/null | head -1)

scp dist/yinmonote-linux-amd64 vpc_tengxun:~/yinmonote/
ssh vpc_tengxun 'sudo systemctl restart yinmonote'
```

Build locally first with `make build`, then run the commands above. The binary is self-contained — no dependency installation on the server is required.

---

## Key Environment Variables

| Variable | Default | Description |
|---|---|---|
| `DATA_DIR` | `~/.yinmonote/notes/` | Notes storage directory |
| `PORT` | `7281` | HTTP listen port |
| `ACME_DOMAIN` | — | Enable Let's Encrypt (set to domain name) |
| `TLS_SELF` | — | Set `1` to enable self-signed TLS |
| `AUTH_USER` | — | Basic auth credential (`user:password` format) |
| `SYNC_COMMIT` | — | Set `1` for immediate git commit on every save (E2E testing only) |
| `E2E_RESET_AUTH` | — | Set `1` to enable auth-reset endpoint (**E2E testing ONLY — never in production**) |

---

## Post-Deploy Checklist

- [ ] TLS configured (ACME, manual cert, self-signed, or reverse proxy termination)
- [ ] `DATA_DIR` points to a persistent volume (not an ephemeral container layer)
- [ ] Regular backups of:
  - `~/.yinmonote/` — application config
  - `DATA_DIR` — all notes and git history
- [ ] WebDAV tested if you plan to use it (`/webdav/` path, same Bearer token auth)
- [ ] `E2E_RESET_AUTH` is **not** set in production
- [ ] Firewall allows only the intended port (default 7281 or 443 for ACME)
