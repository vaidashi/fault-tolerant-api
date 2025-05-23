#!/bin/bash

# This script connects to Kafka and prints out messages being sent to the orders topic
# Requires kafkacat (kcat) to be installed

# if ! command -v kafkacat &> /dev/null; then
#     echo "Error: This script requires kcat (kafkacat) to be installed."
#     echo "On Ubuntu: apt-get install kafkacat"
#     echo "On macOS: brew install kcat"
#     exit 1
# fi

echo "Monitoring Kafka orders topic. Press Ctrl+C to stop."
kafkacat -b localhost:29092 -t orders -C