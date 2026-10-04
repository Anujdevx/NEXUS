# graph-engine: what is thin

- `POST /topology/upload` reads GeoJSON **Point** features only (assets and `depends_on`). LineString roads are not ingested; the road
  graph comes from `frontend/public/roads.json` or the schematic network.
- Neo4j stores the asset graph and, for the schematic network only, `Junction`/`ROAD`. The 167,500-junction OSM network stays in memory.
  `NEAR` relationships therefore exist only when the schematic network is in use.
- The cascade runs in memory and persists statuses afterwards; there is no multi-replica coordination.
- Road state is rebuilt from bus events after a restart (it starts from the opening scenario); it does not read traffic-control's state on boot.
