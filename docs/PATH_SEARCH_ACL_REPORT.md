# Path Search ACL Report Script

`scripts/path_search_acl_report.py` runs Forward path search via Basic Auth and reports one selected path (default: path 1), plus firewall/ACL deny evidence and config lines.

## Required Environment Variables
- `FWD_HOST` (example: `https://fwd.app`)
- `FWD_USER`
- `FWD_PASS`

Optional:
- `FWD_NETWORK_ID` (if `--network-id` is not provided)

## Example
```bash
env \
  FWD_HOST='https://fwd.app' \
  FWD_USER='your-user' \
  FWD_PASS='your-pass' \
  FWD_NETWORK_ID='231060' \
  ./scripts/path_search_acl_report.py \
    --snapshot-id 1175659 \
    --src-ip 10.6.142.197 \
    --dst-ip 10.5.22.96 \
    --proto tcp \
    --dst-port https \
    --path-index 1 \
    --evidence-path-limit 1
```
