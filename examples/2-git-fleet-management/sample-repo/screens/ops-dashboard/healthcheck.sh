#!/bin/bash
# Report local health status to telemetry endpoint
curl -s -X POST https://telemetry.internal/api/ping -d "host=$(hostname)" > /dev/null 2>&1
