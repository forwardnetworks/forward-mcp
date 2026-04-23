#!/usr/bin/env python3
"""
Run Forward path search via Basic Auth and summarize ACL/firewall blocks.

Required environment variables:
  - FWD_HOST (example: https://fwd.app)
  - FWD_USER
  - FWD_PASS
"""

from __future__ import annotations

import argparse
import base64
import json
import os
import re
import ssl
import urllib.error
import urllib.parse
import urllib.request
from typing import Any


PROTO_MAP = {
    "ICMP": 1,
    "TCP": 6,
    "UDP": 17,
}

SERVICE_PORT_MAP = {
    "HTTP": "80",
    "HTTPS": "443",
    "SSH": "22",
    "DNS": "53",
    "NTP": "123",
    "SMTP": "25",
    "SMTPS": "465",
    "IMAP": "143",
    "IMAPS": "993",
    "POP3": "110",
    "POP3S": "995",
    "RDP": "3389",
    "LDAP": "389",
    "LDAPS": "636",
}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Query /api/networks/{networkId}/paths and summarize blocking ACL evidence."
    )
    parser.add_argument("--network-id", type=int, help="Forward network ID (or set FWD_NETWORK_ID)")
    parser.add_argument("--snapshot-id", type=int, help="Forward snapshot ID (optional; defaults to latest PROCESSED)")
    parser.add_argument("--src-ip", required=True, help="Source IP or subnet")
    parser.add_argument("--dst-ip", required=True, help="Destination IP or subnet")
    parser.add_argument("--proto", default="TCP", help="Protocol name or number (default: TCP)")
    parser.add_argument("--src-port", help="Source port/service (example: 443 or HTTPS)")
    parser.add_argument("--dst-port", help="Destination port/service (example: 443 or HTTPS)")
    parser.add_argument("--intent", default="PREFER_DELIVERED", help="Search intent (default: PREFER_DELIVERED)")
    parser.add_argument("--max-results", type=int, default=20, help="Max path results (default: 20)")
    parser.add_argument("--max-seconds", type=int, default=30, help="Search timeout seconds (default: 30)")
    parser.add_argument(
        "--path-index",
        type=int,
        default=1,
        help="1-based path index to report (default: 1)",
    )
    parser.add_argument(
        "--evidence-path-limit",
        type=int,
        default=3,
        help="Max number of blocked paths for deep ACL file/line extraction (default: 3)",
    )
    parser.add_argument(
        "--insecure",
        action="store_true",
        help="Disable TLS cert verification (for labs/self-signed certs)",
    )
    parser.add_argument("--raw-json", action="store_true", help="Print raw API response JSON")
    return parser.parse_args()


def get_env(name: str) -> str:
    value = os.getenv(name)
    if not value:
        raise SystemExit(f"Missing required env var: {name}")
    return value


def normalize_host(host: str) -> str:
    host = host.strip().rstrip("/")
    if not host.startswith(("http://", "https://")):
        host = f"https://{host}"
    return host


def proto_to_number(proto: str) -> int:
    token = str(proto).strip().upper()
    if token.isdigit():
        return int(token)
    if token in PROTO_MAP:
        return PROTO_MAP[token]
    raise SystemExit(f"Unsupported protocol '{proto}'. Use ICMP/TCP/UDP or a number.")


def parse_port(port: str | None) -> str | None:
    if port is None:
        return None
    token = port.strip()
    if not token:
        return None
    if token.isdigit():
        return token
    mapped = SERVICE_PORT_MAP.get(token.upper())
    if mapped:
        return mapped
    raise SystemExit(
        f"Unsupported port '{port}'. Use a number or known service name ({', '.join(sorted(SERVICE_PORT_MAP.keys()))})."
    )


def build_auth_header(username: str, password: str) -> str:
    combined = f"{username}:{password}".encode("utf-8")
    encoded = base64.b64encode(combined).decode("ascii")
    return f"Basic {encoded}"


def api_get_json(
    host: str,
    path: str,
    params: dict[str, Any],
    auth_header: str,
    insecure: bool,
) -> dict[str, Any]:
    query = urllib.parse.urlencode({k: v for k, v in params.items() if v is not None})
    url = f"{host}{path}?{query}"

    req = urllib.request.Request(
        url,
        method="GET",
        headers={
            "Accept": "application/json",
            "Authorization": auth_header,
            "X-Requested-With": "XMLHttpRequest",
        },
    )

    context = None
    if insecure:
        context = ssl.create_default_context()
        context.check_hostname = False
        context.verify_mode = ssl.CERT_NONE

    try:
        with urllib.request.urlopen(req, context=context, timeout=90) as resp:
            payload = resp.read().decode("utf-8")
            return json.loads(payload)
    except urllib.error.HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace")
        raise SystemExit(f"HTTP {exc.code} {exc.reason}\n{body}") from exc
    except urllib.error.URLError as exc:
        raise SystemExit(f"Request failed: {exc}") from exc


def api_post_json(
    host: str,
    path: str,
    params: dict[str, Any],
    body: dict[str, Any],
    auth_header: str,
    insecure: bool,
) -> dict[str, Any]:
    query = urllib.parse.urlencode({k: v for k, v in params.items() if v is not None})
    url = f"{host}{path}?{query}"
    body_bytes = json.dumps(body).encode("utf-8")

    req = urllib.request.Request(
        url,
        data=body_bytes,
        method="POST",
        headers={
            "Accept": "application/json",
            "Content-Type": "application/json",
            "Authorization": auth_header,
            "X-Requested-With": "XMLHttpRequest",
        },
    )

    context = None
    if insecure:
        context = ssl.create_default_context()
        context.check_hostname = False
        context.verify_mode = ssl.CERT_NONE

    try:
        with urllib.request.urlopen(req, context=context, timeout=90) as resp:
            payload = resp.read().decode("utf-8")
            return json.loads(payload)
    except urllib.error.HTTPError as exc:
        body_text = exc.read().decode("utf-8", errors="replace")
        raise SystemExit(f"HTTP {exc.code} {exc.reason}\n{body_text}") from exc
    except urllib.error.URLError as exc:
        raise SystemExit(f"Request failed: {exc}") from exc


def get_network_id(arg_network_id: int | None) -> int:
    if arg_network_id is not None:
        return arg_network_id
    env_id = os.getenv("FWD_NETWORK_ID")
    if env_id and env_id.isdigit():
        return int(env_id)
    raise SystemExit("Missing network ID. Provide --network-id or set FWD_NETWORK_ID.")


def get_latest_processed_snapshot_id(
    host: str,
    network_id: int,
    auth_header: str,
    insecure: bool,
) -> int:
    data = api_get_json(
        host=host,
        path=f"/api/networks/{network_id}/snapshots",
        params={"state": "PROCESSED"},
        auth_header=auth_header,
        insecure=insecure,
    )
    snapshots = data.get("snapshots", [])
    if not snapshots:
        raise SystemExit(f"No PROCESSED snapshots found for network {network_id}.")

    def sort_key(s: dict[str, Any]) -> tuple[str, str, int]:
        return (
            str(s.get("processedAt") or ""),
            str(s.get("createdAt") or ""),
            int(s.get("id") or 0),
        )

    latest = max(snapshots, key=sort_key)
    return int(latest["id"])


def decode_path_query_from_query_url(query_url: str) -> dict[str, Any]:
    parsed = urllib.parse.urlparse(query_url)
    combined = f"{parsed.path}?{parsed.query}"
    match = re.search(r"/paths/([^/?#&]+)", combined)
    if not match:
        match = re.search(r"/paths/([^/?#&]+)", query_url)
    if not match:
        raise SystemExit(f"Could not extract encoded path query from queryUrl: {query_url}")
    encoded = match.group(1)
    padding = "=" * ((4 - len(encoded) % 4) % 4)
    decoded = base64.urlsafe_b64decode(encoded + padding).decode("utf-8")
    return json.loads(decoded)


def build_trace_request(filters: dict[str, Any], selected_path: dict[str, Any] | None, max_seconds: int) -> dict[str, Any]:
    payload: dict[str, Any] = {
        "filters": filters,
        "maxResults": 5000,
        "limit": 0,
        "maxDurationSeconds": max_seconds,
    }
    if selected_path is not None:
        payload["selectedFacets"] = {"paths": [selected_path]}
    return payload


def action_bucket_has_deny(netfn: dict[str, Any]) -> bool:
    for bucket in netfn.get("actionBuckets", []):
        for action in bucket.get("actions", []):
            if str(action.get("type", "")).upper() == "DENY":
                return True
    return False


def format_line_ranges(ranges: list[dict[str, Any]]) -> str:
    return ",".join(f"{int(r['start'])}-{int(r['end'])}" for r in ranges)


def fetch_line_excerpts(
    host: str,
    snapshot_id: int,
    file_name: str,
    ranges: list[dict[str, Any]],
    auth_header: str,
    insecure: bool,
) -> dict[str, Any]:
    return api_get_json(
        host=host,
        path=f"/api/snapshots/{snapshot_id}/files/{urllib.parse.quote(file_name, safe='')}",
        params={"lines": format_line_ranges(ranges)},
        auth_header=auth_header,
        insecure=insecure,
    )


def get_device_files(
    host: str,
    network_id: int,
    snapshot_id: int,
    device_name: str,
    auth_header: str,
    insecure: bool,
) -> list[str]:
    data = api_get_json(
        host=host,
        path=f"/api/networks/{network_id}/devices/{urllib.parse.quote(device_name, safe='')}/files",
        params={"for": "ui", "snapshotId": snapshot_id},
        auth_header=auth_header,
        insecure=insecure,
    )
    return [f.get("fileName") for f in data if isinstance(f, dict) and f.get("fileName")]


def search_file_line_hits(
    host: str,
    snapshot_id: int,
    file_name: str,
    token: str,
    auth_header: str,
    insecure: bool,
) -> list[int]:
    data = api_get_json(
        host=host,
        path=f"/api/snapshots/{snapshot_id}/files/{urllib.parse.quote(file_name, safe='')}/search",
        params={"q": token},
        auth_header=auth_header,
        insecure=insecure,
    )
    return [int(x) for x in data if isinstance(x, int)]


def prioritize_device_files(files: list[str]) -> list[str]:
    def rank(name: str) -> tuple[int, str]:
        lowered = name.lower()
        if lowered.endswith("configuration.txt"):
            return (0, lowered)
        if "security_rules_table" in lowered:
            return (1, lowered)
        if "config_pushed_shared_policy" in lowered:
            return (2, lowered)
        if "policy" in lowered or "rule" in lowered:
            return (3, lowered)
        return (9, lowered)

    return sorted(files, key=rank)


def summarize_paths(response: dict[str, Any], path_index: int) -> list[dict[str, Any]]:
    info = response.get("info", {})
    paths = info.get("paths", [])
    total_hits = info.get("totalHits", {})
    query_url = response.get("queryUrl")
    timed_out = response.get("timedOut")

    print(f"queryUrl: {query_url}")
    print(f"timedOut: {timed_out}")
    print(f"totalHits: {total_hits.get('value')} ({total_hits.get('type')})")
    print(f"returnedPaths: {len(paths)}")
    print("")

    if path_index < 1:
        raise SystemExit("--path-index must be >= 1")
    if not paths:
        print("blockedPathsIdentified: 0")
        print("No denied/blocking ACL evidence found for the returned paths.")
        return []
    if path_index > len(paths):
        raise SystemExit(f"--path-index {path_index} exceeds returnedPaths={len(paths)}")

    idx = path_index
    path = paths[idx - 1]
    forwarding = path.get("forwardingOutcome")
    security = path.get("securityOutcome")
    hops = path.get("hops", [])
    denied_hops: list[tuple[str, str, str]] = []

    for hop in hops:
        device = hop.get("deviceName", "unknown-device")
        behaviors = set(hop.get("behaviors", []))
        acl_entries = ((hop.get("networkFunctions") or {}).get("acl") or [])
        for acl in acl_entries:
            action = (acl.get("action") or "").upper()
            name = acl.get("name") or "<unnamed-acl>"
            context = acl.get("context") or "UNKNOWN"
            if action == "DENY":
                denied_hops.append((device, context, name))

        if "ACL_DENY" in behaviors and not acl_entries:
            denied_hops.append((device, "UNKNOWN", "ACL_DENY (name unavailable in light path output)"))

    blocked_paths: list[dict[str, Any]] = []
    device_path = [str(h.get("deviceName", "unknown-device")) for h in hops]
    path_line = " -> ".join(device_path) if device_path else "<no-hops>"
    print(f"[path {idx}] forwarding={forwarding} security={security} hops={len(hops)}")
    print(f"  path: {path_line}")
    if security == "DENIED" or denied_hops:
        if denied_hops:
            for device, context, name in denied_hops:
                print(f"  - device={device} context={context} rule={name}")
        else:
            print("  - no explicit ACL_DENY entries found in returned hop data")
        blocked_paths.append(
            {
                "index": idx,
                "device_path": device_path,
                "security": security,
                "forwarding": forwarding,
                "denied_hops": denied_hops,
            }
        )
    print("")

    print(f"blockedPathsIdentified: {len(blocked_paths)}")
    if not blocked_paths:
        print("No denied/blocking ACL evidence found for the returned paths.")
    return blocked_paths


def find_matching_trace_facet_paths(
    trace_facets_resp: dict[str, Any],
    blocked_paths: list[dict[str, Any]],
) -> list[dict[str, Any]]:
    facets = (trace_facets_resp.get("facets") or {})
    path_groups = facets.get("pathGroups") or []
    candidates: list[dict[str, Any]] = []
    for group in path_groups:
        trace_type = group.get("traceType")
        for p in group.get("paths", []):
            candidates.append(
                {
                    "path": p.get("path") or [],
                    "traceType": trace_type,
                    "behavior": p.get("behaviorSample"),
                }
            )

    selected: list[dict[str, Any]] = []
    for blocked in blocked_paths:
        wanted = blocked["device_path"]
        match = next((c for c in candidates if c["path"] == wanted), None)
        if match:
            selected.append(
                {
                    "blocked_index": blocked["index"],
                    "selected_path": {
                        "path": match["path"],
                        "traceType": match["traceType"],
                        "behavior": match["behavior"],
                    },
                }
            )
    return selected


def summarize_acl_blocking_evidence(
    host: str,
    network_id: int,
    snapshot_id: int,
    filters: dict[str, Any],
    blocked_trace_selections: list[dict[str, Any]],
    auth_header: str,
    insecure: bool,
    max_seconds: int,
    evidence_path_limit: int,
    blocked_paths: list[dict[str, Any]],
    dst_ip: str,
) -> None:
    if not blocked_trace_selections:
        print("\nNo blocked paths could be matched to trace facets for deep ACL evidence lookup.")
        return

    selections = blocked_trace_selections[: max(1, evidence_path_limit)]
    if len(blocked_trace_selections) > len(selections):
        print(
            f"\nDetailed ACL Config Evidence (showing first {len(selections)} of {len(blocked_trace_selections)} blocked paths):"
        )
    else:
        print("\nDetailed ACL Config Evidence:")
    for item in selections:
        path_idx = item["blocked_index"]
        selected_path = item["selected_path"]
        trace_resp = api_post_json(
            host=host,
            path=f"/api/networks/{network_id}/trace",
            params={"snapshotId": snapshot_id},
            body=build_trace_request(filters, selected_path, max_seconds=max_seconds),
            auth_header=auth_header,
            insecure=insecure,
        )
        device_hops = trace_resp.get("deviceHops", [])
        evidence_found = False

        print(f"- path {path_idx}:")
        for hop in device_hops:
            device = hop.get("device", "unknown-device")
            netfns = hop.get("networkFunctions") or {}
            acl_netfns = netfns.get("ACCESS_CONTROL") or []
            for acl in acl_netfns:
                if not action_bucket_has_deny(acl):
                    continue
                evidence_found = True
                attrs = acl.get("attributes") or {}
                acl_name = (attrs.get("NAME") or ["<unnamed-acl>"])[0]
                acl_context = (attrs.get("ACL_CONTEXT") or ["unknown"])[0]
                print(f"  device={device} acl={acl_name} context={acl_context}")
                file_lines = acl.get("fileLines") or {}
                if not file_lines:
                    print("    fileLines: <none available in trace response>")
                    continue
                for file_name, ranges in file_lines.items():
                    print(f"    file: {file_name}")
                    print(f"    ranges: {format_line_ranges(ranges)}")
                    excerpts = fetch_line_excerpts(
                        host=host,
                        snapshot_id=snapshot_id,
                        file_name=file_name,
                        ranges=ranges,
                        auth_header=auth_header,
                        insecure=insecure,
                    )
                    for excerpt in excerpts.get("excerpts", []):
                        start = int(excerpt["start"])
                        for i, line in enumerate(excerpt.get("lines", [])):
                            print(f"      L{start + i + 1}: {line}")
        if not evidence_found:
            print("  no DENY ACL network function with fileLines found in trace response")
            blocked_meta = next((b for b in blocked_paths if b["index"] == path_idx), None)
            denied_hops = blocked_meta.get("denied_hops", []) if blocked_meta else []
            if denied_hops:
                print("  fallback: searching device config files for blocking rule context")
            found_fallback = False
            for device_name, _, acl_name in denied_hops:
                if "name unavailable" in acl_name.lower():
                    continue
                try:
                    files = get_device_files(
                        host=host,
                        network_id=network_id,
                        snapshot_id=snapshot_id,
                        device_name=device_name,
                        auth_header=auth_header,
                        insecure=insecure,
                    )
                except SystemExit:
                    continue
                files = prioritize_device_files(files)
                for file_name in files[:20]:
                    try:
                        acl_hits = search_file_line_hits(
                            host=host,
                            snapshot_id=snapshot_id,
                            file_name=file_name,
                            token=f"{acl_name} ",
                            auth_header=auth_header,
                            insecure=insecure,
                        )
                        deny_hits = search_file_line_hits(
                            host=host,
                            snapshot_id=snapshot_id,
                            file_name=file_name,
                            token="action deny;",
                            auth_header=auth_header,
                            insecure=insecure,
                        )
                    except SystemExit:
                        continue
                    if not acl_hits:
                        continue

                    picked_acl_line: int | None = None
                    for a in acl_hits:
                        if any(0 <= (d - a) <= 25 for d in deny_hits):
                            picked_acl_line = a
                            break
                    if picked_acl_line is None and deny_hits:
                        picked_acl_line = acl_hits[0]
                    if picked_acl_line is None:
                        continue

                    found_fallback = True
                    print(f"    device={device_name} acl={acl_name} file={file_name}")
                    ranges = [{"start": max(0, picked_acl_line - 2), "end": picked_acl_line + 16}]
                    excerpts = fetch_line_excerpts(
                        host=host,
                        snapshot_id=snapshot_id,
                        file_name=file_name,
                        ranges=ranges,
                        auth_header=auth_header,
                        insecure=insecure,
                    )
                    for excerpt in excerpts.get("excerpts", []):
                        start = int(excerpt["start"])
                        for i, line in enumerate(excerpt.get("lines", [])):
                            print(f"      L{start + i + 1}: {line}")
                    break
            if not found_fallback and denied_hops:
                print("    fallback could not find matching config lines by ACL name/destination IP")


def main() -> None:
    args = parse_args()

    host = normalize_host(get_env("FWD_HOST"))
    network_id = get_network_id(args.network_id)
    username = get_env("FWD_USER")
    password = get_env("FWD_PASS")
    auth = build_auth_header(username, password)
    snapshot_id = args.snapshot_id or get_latest_processed_snapshot_id(
        host=host,
        network_id=network_id,
        auth_header=auth,
        insecure=args.insecure,
    )

    params: dict[str, Any] = {
        "snapshotId": snapshot_id,
        "dstIp": args.dst_ip,
        "srcIp": args.src_ip,
        "intent": args.intent,
        "ipProto": proto_to_number(args.proto),
        "srcPort": parse_port(args.src_port),
        "dstPort": parse_port(args.dst_port),
        "includeNetworkFunctions": "true",
        "maxResults": args.max_results,
        "maxSeconds": args.max_seconds,
    }

    response = api_get_json(
        host=host,
        path=f"/api/networks/{network_id}/paths",
        params=params,
        auth_header=auth,
        insecure=args.insecure,
    )

    if args.raw_json:
        print(json.dumps(response, indent=2, sort_keys=True))
        return

    print(f"networkId: {network_id}")
    print(f"snapshotId: {snapshot_id}")
    blocked = summarize_paths(response, path_index=args.path_index)

    if not blocked:
        return

    query_url = str(response.get("queryUrl") or "")
    filters = decode_path_query_from_query_url(query_url)
    trace_facets_resp = api_post_json(
        host=host,
        path=f"/api/networks/{network_id}/trace-facets",
        params={"snapshotId": snapshot_id, "maxFacetValues": 50},
        body=build_trace_request(filters, selected_path=None, max_seconds=args.max_seconds),
        auth_header=auth,
        insecure=args.insecure,
    )
    blocked_trace_selections = find_matching_trace_facet_paths(trace_facets_resp, blocked)
    summarize_acl_blocking_evidence(
        host=host,
        network_id=network_id,
        snapshot_id=snapshot_id,
        filters=filters,
        blocked_trace_selections=blocked_trace_selections,
        auth_header=auth,
        insecure=args.insecure,
        max_seconds=args.max_seconds,
        evidence_path_limit=args.evidence_path_limit,
        blocked_paths=blocked,
        dst_ip=args.dst_ip,
    )


if __name__ == "__main__":
    main()
