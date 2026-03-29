#!/bin/bash
set -e

NLB="http://a6e1211d45bda4b32b51f6b2d278bce7-1983641930.us-east-2.elb.amazonaws.com"
TOKEN=$(curl -s -X POST "$NLB/v1/auth/login" -H "Content-Type: application/json" \
  -d '{"email":"subsdataqa+3@gmail.com","password":"TestQual@2026!"}' \
  | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('token','') or d.get('AccessToken',''))")

A="Authorization: Bearer $TOKEN"
PID=6057
SID=1191
SVID=41815

check() {
  local label="$1" url="$2"
  local resp
  resp=$(curl -s "$NLB/v1/$url" -H "$A" 2>&1)
  local ok=$(echo "$resp" | python3 -c "
import sys,json
try:
  d=json.load(sys.stdin)
  if d.get('success'):
    data=d.get('data')
    if isinstance(data,list): print(f'OK count={len(data)}')
    elif isinstance(data,dict): print(f'OK keys={list(data.keys())[:5]}')
    else: print(f'OK type={type(data).__name__}')
  else:
    print(json.dumps(d)[:120])
except: print('PARSE_ERROR: '+repr(sys.stdin.read())[:80])
" 2>&1)
  printf "%-45s %s\n" "$label" "$ok"
}

checkpost() {
  local label="$1" url="$2" body="$3"
  local resp
  resp=$(curl -s -X POST "$NLB/v1/$url" -H "$A" -H "Content-Type: application/json" -d "$body" 2>&1)
  local ok=$(echo "$resp" | python3 -c "
import sys,json
try:
  d=json.load(sys.stdin)
  if d.get('success'): print('OK')
  else: print(json.dumps(d)[:120])
except: print('PARSE_ERROR')
" 2>&1)
  printf "%-45s %s\n" "$label" "$ok"
}

echo "============================================"
echo "  Phase 6 Endpoint Test Suite"
echo "============================================"
echo ""

echo "--- Markets ---"
check "GET /markets" "markets"
check "GET /markets/npi" "markets/npi"
check "GET /market/1/crowdable_attributes" "market/1/crowdable_attributes"

echo ""
echo "--- Surveys Extended ---"
check "GET /survey/{id}/detail (LS)" "survey/$SVID/detail?serviceCategory=LS"
check "GET /survey/{id}/validate (LS)" "survey/$SVID/validate?serviceCategory=LS"
check "GET /survey/{id}/crowds (LS)" "survey/$SVID/crowds?serviceCategory=LS"

echo ""
echo "--- Project Sub-Resources (LS) ---"
check "GET /project/{pid}/surveys" "project/$PID/surveys?serviceCategory=LS"
check "GET /project/{pid}/time_slots" "project/$PID/time_slots?serviceCategory=LS"
check "GET /project/{pid}/users" "project/$PID/users?serviceCategory=LS"
check "GET /project/{pid}/observers" "project/$PID/observers?serviceCategory=LS"
check "GET /project/{pid}/interview_media" "project/$PID/interview_media?serviceCategory=LS"
check "GET /project/{pid}/dashboard/avail" "project/$PID/dashboard/availability_and_time_slots?serviceCategory=LS"
check "GET /project/{pid}/availability" "project/$PID/availability?serviceCategory=LS"
check "GET /project/{pid}/scheduler_mods" "project/$PID/scheduler_moderators?serviceCategory=LS"
check "GET /project/{pid}/qual_resched" "project/$PID/qual_resched_body?serviceCategory=LS"

echo ""
echo "--- Subscription (LS) ---"
check "GET /subscription/{id}/interviews" "subscription/$SID/interviews?serviceCategory=LS"
check "GET /subscription/{id}/crowds" "subscription/$SID/crowds?serviceCategory=LS"
check "GET /subscription/{id}/question_types" "subscription/$SID/question_types?serviceCategory=LS"
check "GET /subscription/{subId}/inquiries" "subscription/$SID/inquiries?serviceCategory=LS"
check "GET /subscription/{subId}/project_surveys" "subscription/$SID/project_surveys?serviceCategory=LS"

echo ""
echo "--- Self-Service ---"
check "GET /selfservice/noshow" "selfservice/noshow"

echo ""
echo "--- User ---"
check "GET /user/85 (LS)" "user/85?serviceCategory=LS"

echo ""
echo "--- Payments & Translations ---"
check "GET /payments/status-list" "payments/status-list"
check "GET /translations/locales" "translations/locales"

echo ""
echo "--- Salesforce ---"
check "GET /salesforceprojects" "salesforceprojects"

echo ""
echo "--- Timeslot Sub-Resources ---"
# Find a timeslot with moderators
TSID=$(curl -s "$NLB/v1/project/$PID/time_slots?serviceCategory=LS" -H "$A" | python3 -c "
import sys,json
d=json.load(sys.stdin)
items=d.get('data',[])
print(items[0]['id'] if items else '0')
" 2>&1)
if [ "$TSID" != "0" ] && [ -n "$TSID" ]; then
  check "GET /timeslot/{id}/moderators" "timeslot/$TSID/moderators?serviceCategory=LS"
  check "GET /timeslot/{id}/moderator_options" "timeslot/$TSID/moderator_options_ext?serviceCategory=LS"
  check "GET /timeslot/{id}/observers" "timeslot/$TSID/observers?serviceCategory=LS"
else
  echo "  (No timeslots for project $PID — skipping timeslot sub-resource tests)"
fi

echo ""
echo "--- Moderator Availability ---"
# Test with a known moderator (scheduler_moderators returned 1 for project 6057)
MODID=$(curl -s "$NLB/v1/project/$PID/scheduler_moderators?serviceCategory=LS" -H "$A" | python3 -c "
import sys,json
d=json.load(sys.stdin)
items=d.get('data',[])
print(items[0]['moderatorId'] if items else '0')
" 2>&1)
if [ "$MODID" != "0" ] && [ -n "$MODID" ]; then
  check "GET /moderator/{modId}/availability/{subId}" "moderator/$MODID/availability/$SID"
else
  echo "  (No moderators found — skipping)"
fi

echo ""
echo "============================================"
echo "  Test Complete"
echo "============================================"
