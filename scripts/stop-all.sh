#!/bin/bash

echo "Stopping Microservices..."
for svc in services/*; do
  if [ -d "$svc" ]; then
    echo "Stopping ${svc}..."
    (cd "$svc" && docker-compose down)
  fi
done

echo "Stopping Gateway..."
cd gateway && docker-compose down && cd ..

echo "All services and gateway stopped successfully!"
