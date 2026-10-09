---
name: hexa-ADR-search
description: Search Architecture Decision Records by keyword, status, or date
trigger: /hexa-ADR-search
---

# Search ADRs

## Steps

1. Ask the user what to search for (keyword, status, or date range)

2. Search methods:
   - **By keyword**: `grep -ril "{keyword}" docs/adrs/`
   - **By status**: `grep -rl "Status.*{status}" docs/adrs/`
   - **By date**: `grep -rl "Date.*2026" docs/adrs/`
   - **Via CLI**: `hexa adr search {keyword}`
   - **Via API**: `GET /api/adrs` then filter results

3. Display matching ADRs with:
   - ADR number and title
   - Status badge
   - Matching context snippet

4. Offer to open any result for full reading

## Example

User: `/hexa-ADR-search sqlite`
-> Shows every ADR that mentions SQLite, with relevant snippets
