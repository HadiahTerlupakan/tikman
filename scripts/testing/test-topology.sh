#!/bin/bash

# Get OLT ID from database
OLTS=$(curl -s http://localhost:8080/api/v1/olts)
OLT_ID=$(echo "$OLTS" | python3 -c "import sys, json; olts = json.load(sys.stdin); print(olta.get('id') if (olta := next((o for o in olts), {})) else 'none')" 2>/dev/null || echo "none")

if [ "$OLT_ID" == "none" ] || [ -z "$OLT_ID" ]; then
    echo "No OLT found in database"
    exit 1
fi

echo "Testing topology discovery for OLT: $OLT_ID"
echo ""

# Call topology endpoint
RESPONSE=$(curl -s -X POST "http://localhost:8080/api/v1/olts/$OLT_ID/topology")

# Pretty print first ONT to check fields
echo "$RESPONSE" | python3 -m json.tool | head -50

# Check if name and optical power exist
echo ""
echo "Checking for name and optical power fields..."
echo "$RESPONSE" | python3 -c "
import sys, json
data = json.load(sys.stdin)
topology = data.get('topology', [])
if topology:
    slot = topology[0]
    ports = slot.get('ports', [])
    if ports:
        port = ports[0]
        onts = port.get('onts', [])
        if onts:
            ont = onts[0]
            print(f'Serial Number: {ont.get(\"serial_number\", \"N/A\")}')
            print(f'Name: {ont.get(\"name\", \"N/A\")}')
            print(f'RX Power: {ont.get(\"rx_power\", \"N/A\")}')
            print(f'TX Power: {ont.get(\"tx_power\", \"N/A\")}')
            print(f'Distance: {ont.get(\"distance\", \"N/A\")}')
else:
    print('No topology data returned')
"
