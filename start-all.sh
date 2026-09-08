#!/bin/bash

echo "Starting Gateway..."
cd gateway && docker-compose up -d && cd ..

echo "Starting Microservices..."
for svc in services/*; do
  if [ -d "$svc" ]; then
    echo "Starting ${svc}..."
    (cd "$svc" && docker-compose up -d)
  fi
done

echo "All services and gateway started successfully!"
