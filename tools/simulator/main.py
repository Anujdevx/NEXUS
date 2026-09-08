"""
Nexus Virtual City Telemetry Simulator

A highly concurrent async simulator that generates mock IoT sensor data
and pushes it to the iot-broker service through the Traefik gateway.

Includes a Chaos Generator that injects catastrophic failure payloads
every 30-60 seconds to test cascading failure logic.

Usage:
    python main.py
    
Environment Variables:
    IOT_BROKER_URL  - Base URL for the iot-broker (default: http://localhost/iot-broker)
    INTERVAL_SEC    - Base interval between telemetry pushes (default: 2)
    CHAOS_MIN_SEC   - Minimum chaos interval (default: 30)
    CHAOS_MAX_SEC   - Maximum chaos interval (default: 60)
"""

import asyncio
import json
import os
import random
import time
from datetime import datetime, timezone

import aiohttp

# ── Configuration ───────────────────────────────────────────
IOT_BROKER_URL = os.getenv("IOT_BROKER_URL", "http://localhost/iot-broker")
INGEST_ENDPOINT = f"{IOT_BROKER_URL}/api/iot/ingest"
INTERVAL_SEC = float(os.getenv("INTERVAL_SEC", "2"))
CHAOS_MIN_SEC = int(os.getenv("CHAOS_MIN_SEC", "30"))
CHAOS_MAX_SEC = int(os.getenv("CHAOS_MAX_SEC", "60"))


# ── Sensor Definitions ─────────────────────────────────────
SUBSTATIONS = [
    {"id": f"sub-{i:03d}", "name": f"Substation {chr(65 + i)}", "zone": f"Zone-{i + 1}"}
    for i in range(6)
]

WATER_PUMPS = [
    {"id": f"wp-{i:03d}", "name": f"Pump Station {i + 1}", "zone": f"Zone-{(i % 3) + 1}"}
    for i in range(8)
]

INTERSECTIONS = [
    {"id": f"ix-{i:03d}", "name": f"Intersection {i + 1}", "zone": f"Zone-{(i % 4) + 1}"}
    for i in range(10)
]


# ── Telemetry Generators ───────────────────────────────────
def generate_substation_telemetry(sub: dict, chaos: bool = False) -> dict:
    load = random.uniform(85, 102) if chaos else random.uniform(30, 80)
    return {
        "sensor_type": "power-substation",
        "sensor_id": sub["id"],
        "name": sub["name"],
        "zone": sub["zone"],
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "metrics": {
            "load_pct": round(load, 2),
            "temperature_c": round(random.uniform(35, 95 if chaos else 65), 1),
            "voltage_kv": round(random.uniform(10.5, 13.8), 2),
            "frequency_hz": round(random.uniform(49.5, 50.5), 3),
        },
        "is_critical": load > 100,
    }


def generate_water_pump_telemetry(pump: dict, chaos: bool = False) -> dict:
    pressure = random.uniform(0.2, 1.5) if chaos else random.uniform(3.0, 7.0)
    return {
        "sensor_type": "water-pump",
        "sensor_id": pump["id"],
        "name": pump["name"],
        "zone": pump["zone"],
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "metrics": {
            "flow_rate_lps": round(random.uniform(0.5 if chaos else 10, 50), 2),
            "pressure_bar": round(pressure, 2),
            "motor_temp_c": round(random.uniform(60, 120 if chaos else 80), 1),
        },
        "is_critical": pressure < 1.0,
    }


def generate_intersection_telemetry(ix: dict, chaos: bool = False) -> dict:
    return {
        "sensor_type": "smart-intersection",
        "sensor_id": ix["id"],
        "name": ix["name"],
        "zone": ix["zone"],
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "metrics": {
            "vehicle_count": random.randint(80 if chaos else 5, 200 if chaos else 60),
            "pedestrian_count": random.randint(0, 30),
            "signal_phase": random.choice(["green", "yellow", "red", "flashing" if chaos else "green"]),
            "avg_wait_sec": round(random.uniform(30 if chaos else 5, 180 if chaos else 45), 1),
        },
        "is_critical": chaos,
    }


# ── HTTP Push ───────────────────────────────────────────────
async def push_telemetry(session: aiohttp.ClientSession, payload: dict):
    sensor_type = payload.get("sensor_type", "unknown")
    url = f"{INGEST_ENDPOINT}/{sensor_type}"
    try:
        async with session.post(url, json=payload, timeout=aiohttp.ClientTimeout(total=5)) as resp:
            status = resp.status
            marker = "🔴 CRITICAL" if payload.get("is_critical") else "✅"
            print(f"  {marker} [{status}] {payload['sensor_id']} → {sensor_type} "
                  f"| {json.dumps(payload['metrics'], separators=(',', ':'))}")
    except Exception as e:
        print(f"  ⚠️  Failed to push {payload['sensor_id']}: {e}")


# ── Sensor Loops ────────────────────────────────────────────
async def substation_loop(session: aiohttp.ClientSession, chaos_flag: dict):
    while True:
        tasks = [
            push_telemetry(session, generate_substation_telemetry(sub, chaos=chaos_flag["active"]))
            for sub in SUBSTATIONS
        ]
        await asyncio.gather(*tasks)
        await asyncio.sleep(INTERVAL_SEC + random.uniform(0, 1))


async def water_pump_loop(session: aiohttp.ClientSession, chaos_flag: dict):
    while True:
        tasks = [
            push_telemetry(session, generate_water_pump_telemetry(pump, chaos=chaos_flag["active"]))
            for pump in WATER_PUMPS
        ]
        await asyncio.gather(*tasks)
        await asyncio.sleep(INTERVAL_SEC + random.uniform(0, 1.5))


async def intersection_loop(session: aiohttp.ClientSession, chaos_flag: dict):
    while True:
        tasks = [
            push_telemetry(session, generate_intersection_telemetry(ix, chaos=chaos_flag["active"]))
            for ix in INTERSECTIONS
        ]
        await asyncio.gather(*tasks)
        await asyncio.sleep(INTERVAL_SEC + random.uniform(0, 2))


# ── Chaos Generator ────────────────────────────────────────
async def chaos_generator(chaos_flag: dict):
    """Every 30-60 seconds, activate chaos mode for 5-10 seconds."""
    while True:
        wait = random.randint(CHAOS_MIN_SEC, CHAOS_MAX_SEC)
        await asyncio.sleep(wait)
        duration = random.randint(5, 10)
        print(f"\n🔥 CHAOS ACTIVE for {duration}s — injecting catastrophic failures...\n")
        chaos_flag["active"] = True
        await asyncio.sleep(duration)
        chaos_flag["active"] = False
        print(f"\n✅ Chaos subsided. Returning to nominal telemetry.\n")


# ── Main ────────────────────────────────────────────────────
async def main():
    print("=" * 60)
    print("  Nexus Virtual City Telemetry Simulator")
    print(f"  Target: {IOT_BROKER_URL}")
    print(f"  Sensors: {len(SUBSTATIONS)} substations, "
          f"{len(WATER_PUMPS)} pumps, {len(INTERSECTIONS)} intersections")
    print(f"  Chaos interval: {CHAOS_MIN_SEC}-{CHAOS_MAX_SEC}s")
    print("=" * 60)

    chaos_flag = {"active": False}

    async with aiohttp.ClientSession() as session:
        await asyncio.gather(
            substation_loop(session, chaos_flag),
            water_pump_loop(session, chaos_flag),
            intersection_loop(session, chaos_flag),
            chaos_generator(chaos_flag),
        )


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        print("\nSimulator stopped.")
