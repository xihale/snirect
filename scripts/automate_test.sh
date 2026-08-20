#!/bin/bash
# Developer smoke test: rebuild core & debug APK, install, curl a few URLs.
# Needs adb and a connected device/emulator.
set -e
cd "$(dirname "$0")/.."

echo "Building core and Debug APK..."
make debug

echo "Installing to device..."
make install BUILD_TYPE=Debug

# Launch the app
echo "Launching Snirect (Restarting Process)..."
adb shell am force-stop com.xihale.snirect.debug
adb shell am start -n com.xihale.snirect.debug/com.xihale.snirect.MainActivity >/dev/null

echo "Waiting for VPN to establish..."
sleep 2

# Verify Connectivity via ADB (Requires device connected)
echo "----------------------------------------"
echo "VERIFYING CONNECTIVITY via ADB shell curl"
echo "----------------------------------------"

check_url() {
    local url=$1
    local expected=$2
    echo "Checking $url (Expected: $expected)..."
    adb shell "curl -k -I -m 5 -s -o /dev/null -w '%{http_code}' '$url'" || echo "FAIL (Command Error)"
    echo ""
}

# Check Google (Should be 200 or 301/302)
check_url "https://www.google.com" "200/3xx"

# Check DuckDuckGo (Should be 200 or 301/302)
check_url "https://duckduckgo.com" "200/3xx"

# Check Pixiv (Should be 200 or 301/302)
check_url "https://www.pixiv.net" "200/3xx"

echo "----------------------------------------"
echo "Monitoring logs for 10 seconds..."
timeout 10 adb logcat -v time -s Snirect:* GoLog:* || true
echo "Monitoring finished."
