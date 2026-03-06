# deploy-ssh

Deploy files to a remote server via SSH/SCP.

## Usage

```toml
[tasks.deploy]
uses = "dagryn/deploy-ssh@v1.0.0"
with = { host = "example.com", user = "deploy", source = "./dist", target = "/var/www/app" }
```

## Inputs

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| host | Yes | - | Remote host to deploy to |
| user | Yes | - | SSH user for the remote host |
| source | Yes | - | Local source path to deploy |
| target | Yes | - | Remote target path |
| key | No | ~/.ssh/id_rsa | Path to SSH private key |
| port | No | 22 | SSH port |
| pre-deploy-command | No | - | Command to run on the remote host before deploying |
| post-deploy-command | No | - | Command to run on the remote host after deploying |

## Steps

1. **check-ssh** - Verify SSH key exists and test connection
2. **pre-deploy** - Run optional pre-deploy command on remote host
3. **deploy** - Copy files via SCP
4. **post-deploy** - Run optional post-deploy command on remote host
