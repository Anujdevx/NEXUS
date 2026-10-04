#!/usr/bin/env python3
"""
NEXUS simulator: drives the running backend so the demo has something to react to.
Everything it produces is tagged source "simulated".

  python tools/simulator/main.py telemetry              utility load, rainfall and river levels, once a second
  python tools/simulator/main.py sos-burst --n 10       realistic SOS requests at random flood-prone localities
  python tools/simulator/main.py chaos --asset sub-01   push a substation over 110% until the cascade fires
  python tools/simulator/main.py monsoon                a 3-minute scripted storm: rain, flood stage and rivers rise together

Connection (all optional; defaults read from the repo's .env and .run/ports.env):
  --url         gateway base URL            default http://localhost:$GATEWAY_PORT (8000)
  --service-key X-Service-Key for ingest    default SERVICE_KEY from .env
  --transport   http | amqp                 how telemetry reaches the bus (amqp publishes straight to RabbitMQ with pika)
"""
import argparse
import json
import math
import os
import random
import sys
import time
import uuid
from datetime import datetime, timezone

import requests

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
SEED = os.path.join(ROOT, "tools", "seed", "out")
SRC = "simulated"
ORIGIN = "simulator"


# ---------------------------------------------------------------- config
def read_env(path):
    out = {}
    try:
        for line in open(path):
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                k, v = line.split("=", 1)
                out[k.strip()] = v.strip()
    except OSError:
        pass
    return out


ENV = {**read_env(os.path.join(ROOT, ".env")), **read_env(os.path.join(ROOT, ".run", "ports.env")), **os.environ}


def now():
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


class Nexus:
    """Small client for the gateway: demo tokens, ingest, SOS and a few reads."""

    def __init__(self, url, key, transport="http"):
        self.url = url.rstrip("/")
        self.key = key
        self.transport = transport
        self.s = requests.Session()
        self.tokens = {}
        self.amqp = None

    def token(self, role):
        if role not in self.tokens:
            r = self.s.post(f"{self.url}/api/v1/auth/demo-token", json={"role": role}, timeout=5)
            r.raise_for_status()
            self.tokens[role] = r.json()["token"]
        return self.tokens[role]

    def auth(self, role):
        return {"Authorization": f"Bearer {self.token(role)}"}

    def get(self, path, role="Controller"):
        r = self.s.get(f"{self.url}/api/v1{path}", headers=self.auth(role), timeout=8)
        r.raise_for_status()
        return r.json()

    def ingest(self, readings):
        """Send telemetry readings: {asset_id, type, value}. HTTP ingest, or straight onto RabbitMQ with pika."""
        items = [{**r, "timestamp": now(), "source": SRC} for r in readings]
        if self.transport == "amqp":
            return self._amqp(items)
        r = self.s.post(f"{self.url}/api/v1/telemetry/ingest", json={"items": items}, headers={"X-Service-Key": self.key}, timeout=5)
        r.raise_for_status()

    def _amqp(self, items):
        import pika  # imported here so the HTTP transport needs no broker library at runtime

        if self.amqp is None or self.amqp.is_closed:
            user, pw = ENV.get("RABBITMQ_USER", "nexus"), ENV.get("RABBITMQ_PASS", "nexus-rabbit")
            port = int(ENV.get("RABBITMQ_PORT", "5672"))
            self.amqp = pika.BlockingConnection(pika.ConnectionParameters("localhost", port, "/", pika.PlainCredentials(user, pw)))
            self.ch = self.amqp.channel()
            self.ch.exchange_declare("nexus.events", "topic", durable=True)
        for it in items:
            env = {"id": str(uuid.uuid4()), "entity": "telemetry", "type": "raw", "geo": None, "status": None, "capacity": None,
                   "confidence": 1, "timestamp": it["timestamp"], "source": SRC, "origin": ORIGIN, "payload": it}
            self.ch.basic_publish("nexus.events", "telemetry.raw", json.dumps(env),
                                  pika.BasicProperties(content_type="application/json", delivery_mode=2))

    def sos(self, body):
        r = self.s.post(f"{self.url}/api/v1/sos", json=body, headers=self.auth("Citizen"), timeout=8)
        r.raise_for_status()
        return r.json()


# ---------------------------------------------------------------- the world being simulated
UTILITIES = [  # id, telemetry type, normal load %
    ("sub-01", "power_load_pct", 62), ("sub-02", "power_load_pct", 62), ("sub-03", "power_load_pct", 62),
    ("wp-01", "water_load_pct", 45), ("wp-02", "water_load_pct", 45), ("wp-03", "water_load_pct", 45), ("wp-04", "water_load_pct", 45),
    ("twr-01", "telecom_load_pct", 35), ("twr-02", "telecom_load_pct", 35), ("twr-03", "telecom_load_pct", 35),
]
STATIONS = ["Sahastradhara", "Maldevta", "Hathi Barkala", "Jolly Grant", "Kalsi"]
GAUGES = {"ganga-rishikesh": 3.2, "tons-tapkeshwar": 1.1, "song-maldevta": 0.9, "rispana-city": 0.6}  # normal level, metres


def jitter(v, pct):
    return v * (1 + random.uniform(-pct, pct))


def normal_tick():
    r = []
    for aid, typ, base in UTILITIES:
        r.append({"asset_id": aid, "type": typ, "value": round(max(5, min(88, jitter(base, 0.12))), 1)})  # always below the failure threshold
    for st in STATIONS:
        r.append({"asset_id": st, "type": "rainfall_mm", "value": round(random.uniform(0, 4), 1)})
    for g, base in GAUGES.items():
        r.append({"asset_id": g, "type": "river_level_m", "value": round(jitter(base, 0.04), 2)})
    return r


# ---------------------------------------------------------------- modes
def cmd_telemetry(n, a):
    print(f"telemetry: {len(normal_tick())} readings per tick, every ~{a.interval}s, transport={a.transport}, tagged {SRC}. Ctrl-C to stop.")
    ticks = 0
    try:
        while a.ticks == 0 or ticks < a.ticks:
            n.ingest(normal_tick())
            ticks += 1
            if ticks % 10 == 1:
                print(f"  tick {ticks}")
            time.sleep(max(0.2, jitter(a.interval, 0.2)))
    except KeyboardInterrupt:
        print("stopped")


def cmd_sos_burst(n, a):
    flood = json.load(open(os.path.join(SEED, "FLOOD.json")))
    hazards = ["Flood", "Landslide", "Trapped", "Medical"]
    ids = []
    for i in range(a.n):
        f = random.choice(flood)
        sid = f"SOS-{random.randint(10000, 99999)}"
        body = {"id": sid, "place": f["id"], "placeName": f["n"], "node": f["node"], "lat": f["lat"], "lng": f["lng"],
                "hazard": hazards[i % 4], "people": random.randint(1, 6), "injured": "Yes" if random.random() < 0.4 else "No",
                "via": "direct", "client": ORIGIN}
        r = n.sos(body)
        ids.append(sid)
        print(f"  {i + 1:>2}. {sid}  {body['hazard']:<9} {body['people']} people, injured {body['injured']:<3} at {f['n']}  -> {r['status']}")
        time.sleep(a.gap)
    time.sleep(2.5)
    inc = {i["id"]: i for i in n.get("/incidents")["items"]}
    got = [inc[s] for s in ids if s in inc]
    assigned = [i for i in got if i["status"] == "Unit assigned"]
    print(f"{len(got)} incidents recorded, {len(assigned)} with an ambulance assigned ({len(got) - len(assigned)} queued: no unit free)")


def cmd_chaos(n, a):
    assets = {x["id"]: x for x in n.get("/topology/assets")["items"]}
    if a.asset not in assets:
        sys.exit(f"unknown asset {a.asset}. Try one of: {', '.join(list(assets)[:12])}")
    x = assets[a.asset]
    grace = x["grace_period_ms"] / 1000
    typ = {"POWER": "power_load_pct", "WATER": "water_load_pct", "TELECOM": "telecom_load_pct"}.get(x["type"], "load_pct")
    print(f"chaos: ramping {a.asset} ({x['name']}) from {x['load_percentage']:.0f}% to over 110%; failure threshold {x['failure_threshold']:.0f}%, grace {grace:.0f}s")
    load, t0, over_since = x["base_load"], time.time(), None
    for step in range(int(a.timeout / 0.5)):
        load = min(118, load + random.uniform(2.5, 4.0)) if load < 112 else random.uniform(112, 118)
        n.ingest([{"asset_id": a.asset, "type": typ, "value": round(load, 1)}])
        if load > x["failure_threshold"] and over_since is None:
            over_since = time.time()
        held = f", over the threshold for {time.time() - over_since:.1f}s" if over_since else ""
        print(f"  {time.time() - t0:5.1f}s  load {load:6.1f}%{held}")
        time.sleep(0.5)
        cur = {y["id"]: y for y in n.get("/topology/assets")["items"]}
        if cur[a.asset]["status"] == "FAILED":
            failed = [y for y in cur.values() if y["status"] == "FAILED"]
            degraded = [y for y in cur.values() if y["status"] == "DEGRADED"]
            hosp = [y["name"] for y in failed + degraded if y["type"] == "HOSPITAL"]
            print(f"\ncascade fired: {len(failed)} failed, {len(degraded)} degraded.")
            print(f"hospitals now diverting ({len(hosp)}): {', '.join(hosp[:6])}{' ...' if len(hosp) > 6 else ''}")
            print("Open Infrastructure in the console. New SOS requests now route to hospitals that still have power.")
            print(f"Heal everything: POST /api/v1/topology/reset  (or the 'Restore utilities' button).")
            return
    print("timed out: the asset did not fail. Is graph-engine up and consuming telemetry.raw?")


def smooth(x):
    return x * x * (3 - 2 * x)


def cmd_monsoon(n, a):
    """A scripted storm. Peak rainfall per station differs; flood stage and river levels follow the rain."""
    peak = {"Sahastradhara": 192, "Maldevta": 142, "Hathi Barkala": 93, "Jolly Grant": 93, "Kalsi": 84}  # shaped like 15-16 Sep 2025
    gpeak = {"ganga-rishikesh": 6.4, "tons-tapkeshwar": 3.8, "song-maldevta": 3.3, "rispana-city": 2.7}
    dur = a.duration
    print(f"monsoon: {dur:.0f}s storm. Roads close around 80 mm and 150 mm, the flood stage climbs to 100%, rivers pass their warning and danger marks.")
    t0, step, last = time.time(), 0, -1
    while True:
        t = time.time() - t0
        p = min(1.0, t / dur)
        r = smooth(p)
        readings = []
        for st, pk in peak.items():
            readings.append({"asset_id": st, "type": "rainfall_mm", "value": round(pk * r * random.uniform(0.97, 1.03), 1)})
        for g, pk in gpeak.items():
            readings.append({"asset_id": g, "type": "river_level_m", "value": round(GAUGES[g] + (pk - GAUGES[g]) * smooth(max(0, (p - 0.15) / 0.85)), 2)})
        readings.append({"asset_id": "district", "type": "flood_stage", "value": round(100 * smooth(max(0, (p - 0.2) / 0.8)), 1)})
        n.ingest(readings)
        if int(t) // 10 != last:
            last = int(t) // 10
            mx = max(x["value"] for x in readings if x["type"] == "rainfall_mm")
            fs = [x["value"] for x in readings if x["type"] == "flood_stage"][0]
            print(f"  t+{t:5.0f}s  max rain {mx:6.1f} mm   flood stage {fs:5.1f}%")
        if p >= 1.0:
            break
        time.sleep(a.interval)
    print("storm peaked. Early warning shows the inferred closures; routes now avoid them. Re-run telemetry to let it ease.")


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--url", default=f"http://localhost:{ENV.get('GATEWAY_PORT', '8000')}")
    ap.add_argument("--service-key", default=ENV.get("SERVICE_KEY", "dev-service-key"))
    ap.add_argument("--transport", choices=["http", "amqp"], default="http")
    ap.add_argument("--seed", type=int, default=None, help="random seed, for repeatable runs")
    sub = ap.add_subparsers(dest="mode", required=True)
    t = sub.add_parser("telemetry", help="utility load, rainfall and river levels")
    t.add_argument("--interval", type=float, default=1.0)
    t.add_argument("--ticks", type=int, default=0, help="stop after this many ticks (0 = run until Ctrl-C)")
    b = sub.add_parser("sos-burst", help="a burst of SOS requests")
    b.add_argument("--n", type=int, default=10)
    b.add_argument("--gap", type=float, default=0.4, help="seconds between requests")
    c = sub.add_parser("chaos", help="overload an asset to trigger the cascade")
    c.add_argument("--asset", required=True, help="asset id, e.g. sub-01")
    c.add_argument("--timeout", type=float, default=60)
    m = sub.add_parser("monsoon", help="a scripted storm")
    m.add_argument("--duration", type=float, default=180)
    m.add_argument("--interval", type=float, default=2.0)
    a = ap.parse_args()
    if a.seed is not None:
        random.seed(a.seed)
    n = Nexus(a.url, a.service_key, a.transport)
    try:
        {"telemetry": cmd_telemetry, "sos-burst": cmd_sos_burst, "chaos": cmd_chaos, "monsoon": cmd_monsoon}[a.mode](n, a)
    except requests.RequestException as e:
        sys.exit(f"cannot reach NEXUS at {a.url}: {e}\nIs the backend up? scripts/start-all.sh")


if __name__ == "__main__":
    main()
