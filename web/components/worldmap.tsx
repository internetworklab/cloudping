"use client";

import { Fragment, useEffect } from "react";
import worldMapAny from "./worldmap.json";
import { Box } from "@mui/material";

// format: [longitude, latitude]
type LonLat = number[];

type Polygon = LonLat[];

type Geometry = {
    type: "Polygon" | "MultiPolygon";
    coordinates: Polygon | Polygon[][] | Polygon[][][] | Polygon[][][][][];
}

type Feature = {
  type: "Feature";
  geometry: Geometry;
  properties: Record<string, any>;
};

type FeatureCollection = {
  type: "FeatureCollection";
  features: Feature[];
};

function isPolygon(polygon: any): boolean {
  if (Array.isArray(polygon)) {
    if (polygon.length > 0) {
      if (Array.isArray(polygon[0])) {
        if (polygon[0].length === 2) {
          if (
            typeof polygon[0][0] === "number" &&
            typeof polygon[0][1] === "number"
          ) {
            return true;
          }
        }
      }
    }
  }
  return false;
}

function* yieldPolygons(
  polygonOrPolygons: any
): Generator<Polygon, void, unknown> {
  if (isPolygon(polygonOrPolygons)) {
    yield polygonOrPolygons as Polygon;
  } else if (Array.isArray(polygonOrPolygons)) {
    for (const polygonany of polygonOrPolygons) {
      for (const polygonx of yieldPolygons(polygonany)) {
        yield polygonx;
      }
    }
  }
}

export function WorldMap() {
  useEffect(() => {
    const worldMap = worldMapAny as FeatureCollection;
    let fuck = 0;
    for (const feature of worldMap.features) {
      const geometry = feature.geometry;
      const name = feature.properties?.name;
      if (name === undefined || name === null || name === "") {
        continue;
      }
      for (const polygon of yieldPolygons(geometry.coordinates)) {
        const ispol = isPolygon(polygon);
        console.log("[dbg] country", name, "polygon", ispol);
        if (!ispol) {
          fuck++;
        }
      }
    }
    console.log("[dbg fuck", fuck);
  });
  return (
    <Fragment>
      <Box sx={{ height: "100%" }}>Test Content</Box>
    </Fragment>
  );
}
