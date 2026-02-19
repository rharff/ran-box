#!/usr/bin/env bash
# =============================================================================
# Naratel Box API — Manual Test Script
# Usage: bash test-api.sh
# Requirements: curl, jq
# =============================================================================

set -euo pipefail

BASE_URL="http://localhost:8080/api/v1"
TOKEN=""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

ok()   { echo -e "${GREEN}✅ $1${NC}"; }
fail() { echo -e "${RED}❌ $1${NC}"; }
info() { echo -e "${CYAN}▶  $1${NC}"; }
sep()  { echo -e "\n────────────────────────────────────────"; }

# ── 0. Health Check ──────────────────────────────────────────────────────────
sep
info "0. Health Check"
HEALTH=$(curl -sf "${BASE_URL%/api/v1}/health")
echo "$HEALTH" | jq .
[[ $(echo "$HEALTH" | jq -r '.status') == "ok" ]] && ok "Server is healthy" || fail "Server unreachable"

# ── 1. Register ──────────────────────────────────────────────────────────────
sep
info "1. Register new user"
REGISTER=$(curl -sf -X POST "$BASE_URL/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"email":"test@naratel.id","password":"password123"}' || true)
echo "$REGISTER" | jq . 2>/dev/null || echo "$REGISTER"

# ── 2. Login ─────────────────────────────────────────────────────────────────
sep
info "2. Login"
LOGIN=$(curl -sf -X POST "$BASE_URL/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"test@naratel.id","password":"password123"}')
echo "$LOGIN" | jq .
TOKEN=$(echo "$LOGIN" | jq -r '.token')
[[ -n "$TOKEN" && "$TOKEN" != "null" ]] && ok "Token acquired" || { fail "Login failed"; exit 1; }

# ── 3. List Files (empty) ────────────────────────────────────────────────────
sep
info "3. List files (expect empty)"
curl -sf "$BASE_URL/files" \
  -H "Authorization: Bearer $TOKEN" | jq .
ok "List files OK"

# ── 4. Upload a file ─────────────────────────────────────────────────────────
sep
info "4. Upload test file (generating 10MB random file)"
TEST_FILE="/tmp/naratel_test_$(date +%s).bin"
dd if=/dev/urandom of="$TEST_FILE" bs=1M count=10 2>/dev/null
ok "Test file created: $TEST_FILE (10MB)"

info "Uploading..."
UPLOAD=$(curl -sf -X POST "$BASE_URL/files" \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@$TEST_FILE")
echo "$UPLOAD" | jq .
FILE_ID=$(echo "$UPLOAD" | jq -r '.file_id')
[[ -n "$FILE_ID" && "$FILE_ID" != "null" ]] && ok "Uploaded! File ID: $FILE_ID" || { fail "Upload failed"; exit 1; }

# ── 5. Upload same file again (dedup check) ──────────────────────────────────
sep
info "5. Upload SAME file again (dedup — blocks_count should be same, new file_id)"
UPLOAD2=$(curl -sf -X POST "$BASE_URL/files" \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@$TEST_FILE")
echo "$UPLOAD2" | jq .
ok "Second upload complete — verify blocks_count matches first upload (dedup working)"

# ── 6. List Files ────────────────────────────────────────────────────────────
sep
info "6. List files (expect 2 entries)"
curl -sf "$BASE_URL/files" \
  -H "Authorization: Bearer $TOKEN" | jq .
ok "List files OK"

# ── 7. Download file ─────────────────────────────────────────────────────────
sep
info "7. Download file (ID: $FILE_ID)"
DOWNLOAD_PATH="/tmp/naratel_download_$(date +%s).bin"
HTTP_CODE=$(curl -s -o "$DOWNLOAD_PATH" -w "%{http_code}" \
  -H "Authorization: Bearer $TOKEN" \
  "$BASE_URL/files/$FILE_ID")
if [[ "$HTTP_CODE" == "200" ]]; then
  ORIG_SIZE=$(stat -c%s "$TEST_FILE")
  DOWN_SIZE=$(stat -c%s "$DOWNLOAD_PATH")
  [[ "$ORIG_SIZE" == "$DOWN_SIZE" ]] && ok "Downloaded! Size matches ($DOWN_SIZE bytes)" || fail "Size mismatch: original=$ORIG_SIZE downloaded=$DOWN_SIZE"
else
  fail "Download failed with HTTP $HTTP_CODE"
fi

# ── 8. Unauthorized access (wrong user) ──────────────────────────────────────
sep
info "8. Test 403 — access without token"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/files/$FILE_ID")
[[ "$HTTP_CODE" == "401" ]] && ok "Correctly rejected with 401" || fail "Expected 401, got $HTTP_CODE"

# ── 9. Delete file ────────────────────────────────────────────────────────────
sep
info "9. Delete file (ID: $FILE_ID)"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE \
  -H "Authorization: Bearer $TOKEN" \
  "$BASE_URL/files/$FILE_ID")
[[ "$HTTP_CODE" == "204" ]] && ok "File deleted (204 No Content)" || fail "Delete failed with HTTP $HTTP_CODE"

# ── 10. Confirm deletion ─────────────────────────────────────────────────────
sep
info "10. Download deleted file (expect 403)"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer $TOKEN" \
  "$BASE_URL/files/$FILE_ID")
[[ "$HTTP_CODE" == "403" ]] && ok "Correctly got 403 after deletion" || fail "Expected 403, got $HTTP_CODE"

# ── Cleanup ───────────────────────────────────────────────────────────────────
sep
rm -f "$TEST_FILE" "$DOWNLOAD_PATH"
ok "All tests complete! Check Swagger UI at: http://localhost:8080/swagger/index.html"
