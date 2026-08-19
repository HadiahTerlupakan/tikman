#!/bin/bash
# Test script untuk verifikasi traffic statistics collection

echo "========================================"
echo "Traffic Statistics Collection Test"
echo "========================================"
echo ""

# Check if OLT environment variables are set
if [ -z "$OLT_IP" ]; then
    echo "⚠️  OLT_IP not set, using default: 172.20.1.251"
    export OLT_IP="172.20.1.251"
fi

if [ -z "$OLT_COMMUNITY" ]; then
    echo "⚠️  OLT_COMMUNITY not set, using default: public"
    export OLT_COMMUNITY="public"
fi

echo "Test Configuration:"
echo "  OLT IP: $OLT_IP"
echo "  Community: $OLT_COMMUNITY"
echo ""
echo "Building test program..."

cd backend
go build -o test_traffic_stats test_traffic_stats.go

if [ $? -ne 0 ]; then
    echo "❌ Build failed"
    exit 1
fi

echo "✅ Build successful"
echo ""
echo "Running test..."
echo "========================================"
echo ""

./test_traffic_stats

TEST_RESULT=$?

echo ""
echo "========================================"
if [ $TEST_RESULT -eq 0 ]; then
    echo "✅ TEST PASSED"
else
    echo "❌ TEST FAILED (exit code: $TEST_RESULT)"
fi
echo "========================================"

exit $TEST_RESULT
