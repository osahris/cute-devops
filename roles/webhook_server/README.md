<!--
SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Webhook Server Role

This role sets up a webhook server that can trigger deployments via HTTP requests.

## Features

- Runs as a dedicated non-root user (`webhook`)
- Uses sudo to execute the deploy command
- Supports HMAC-SHA256 authentication for secure webhook calls
- Systemd service with automatic restart on failure
- Configurable port and listening address

## Configuration

Default variables (can be overridden):

```yaml
webhook_user: webhook
webhook_group: webhook
webhook_port: 9000
webhook_listen_address: "0.0.0.0"
webhook_auth_user: deploy
webhook_auth_password: "changeme"  # Should be vaulted in production
```

## Testing the Webhook

To test the webhook deployment:

```bash
PAYLOAD='{"action":"deploy","instance":"ohai"}'
SECRET="changeme"
SIGNATURE=$(echo -n "$PAYLOAD" | openssl dgst -sha256 -hmac "$SECRET" | cut -d' ' -f2)

curl -X POST \
  http://localhost:9000/hooks/deploy-instance \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Signature: sha256=$SIGNATURE" \
  -d "$PAYLOAD"
```

## Security Notes

- The webhook user has sudo access only to the `/usr/local/bin/deploy` command
- Authentication uses HMAC-SHA256 with a shared secret
- The systemd service has been configured to allow sudo execution
- In production, use a strong password and store it in Ansible Vault

## Webhook Endpoint

The webhook listens on `http://HOST:9000/hooks/deploy-instance`

Required payload format:
```json
{
  "action": "deploy",
  "instance": "instance-name"
}
```