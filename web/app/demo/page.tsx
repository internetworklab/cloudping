"use client";

import {
  LonLat,
  Marker,
  Path,
  useCanvasSizing,
  WorldMap,
} from "@/components/worldmap";
import { Box } from "@mui/material";
import { CSSProperties, Fragment, useEffect, useState } from "react";
import { Quaternion, Vector3, Euler } from "three";

function getQuatFromLat(lat: number): Quaternion {
  const latQuat = new Quaternion();
  latQuat.setFromAxisAngle(new Vector3(1, 0, 0), (-1 * lat * Math.PI) / 180);
  return latQuat;
}

function getQuatFromLon(lon: number): Quaternion {
  const lonQuat = new Quaternion();
  lonQuat.setFromAxisAngle(new Vector3(0, 1, 0), (lon * Math.PI) / 180);
  return lonQuat;
}

function latLonToQuat(lat: number, lon: number): Quaternion {
  const latQuat = getQuatFromLat(lat);
  const lonQuat = getQuatFromLon(lon);

  return lonQuat.clone().multiply(latQuat);
}

const baseVector = new Vector3(0, 0, 1);

type City = {
  name: string;
  latLon: [number, number];
};

function xyzToLatLon(xyz: Vector3): [number, number] {
  const x = xyz.x;
  const y = xyz.y;
  const z = xyz.z;

  const lat = (Math.atan(y / Math.sqrt(z * z + x * x)) * 180) / Math.PI;
  const lonDelta = (Math.atan(z / x) * 180) / Math.PI;
  let lon = lonDelta;
  if (x >= 0) {
    lon = 90 - lon;
  } else {
    lon = -1 * (90 + lon);
  }
  return [lat, lon];
}

function getRelativeQuat(q1: Quaternion, q2: Quaternion): Quaternion {
  return q1.clone().invert().multiply(q2);
}

function getGeodesicPoints(
  startPoint: Vector3,
  endPoint: Vector3,
  numPoints: number
): Vector3[] {
  const points = [];
  const quaternion = new Quaternion();

  // Calculate the quaternion representing the full rotation
  quaternion.setFromUnitVectors(startPoint, endPoint);

  for (let i = 0; i <= numPoints; i++) {
    const t = i / numPoints;
    const interpolatedQuaternion = new Quaternion()
      .identity()
      .slerp(quaternion, t);

    // Apply the interpolated quaternion to the starting point
    const geodesicPoint = startPoint
      .clone()
      .applyQuaternion(interpolatedQuaternion);

    // If your sphere has a specific radius, multiply the point's components by that radius
    // const radius = 10;
    // geodesicPoint.multiplyScalar(radius);

    points.push(geodesicPoint);
  }

  return points;
}

function markVector3(points: LonLat[]): [number, number] | undefined {
  for (let j = 1; j < points.length; j++) {
    const i = j - 1;
    const lon1 = points[i][0] + 180;
    const lon2 = points[j][0] + 180;
    if (
      Math.sin((lon1 * Math.PI) / 180) * Math.sin((lon2 * Math.PI) / 180) <
      0
    ) {
      return [i, j];
    }
  }

  return undefined;
}

function toGeodesicPaths(from: City, to: City, numPoints: number): Path[] {
  const xyzFrom = baseVector
    .clone()
    .applyQuaternion(latLonToQuat(from.latLon[0], from.latLon[1]));

  const xyzTo = baseVector
    .clone()
    .applyQuaternion(latLonToQuat(to.latLon[0], to.latLon[1]));

  const vpoints = getGeodesicPoints(xyzFrom.clone(), xyzTo.clone(), numPoints);
  const lonLats = vpoints.map(xyzToLatLon).map((x) => [x[1], x[0]]);
  const markIndices = markVector3(lonLats);
  const paths: Path[] = [];

  if (markIndices !== undefined) {
    const lonLats1 = lonLats.slice(0, markIndices[0] + 1);
    if (lonLats1.length > 1) {
      paths.push({
        points: lonLats1,
      });
    }

    const lonLats2 = lonLats.slice(markIndices[0] + 1);
    if (lonLats2.length > 1) {
      paths.push({
        points: lonLats2,
      });
    }
    return paths;
  }
  return [
    {
      points: lonLats,
    },
  ];
}

export default function DemoPage() {
  const [show, setShow] = useState(false);
  //   useEffect(() => {
  //     const ticker = window.setInterval(() => {
  //       setShow((prev) => !prev);
  //     }, 250);
  //     return () => {
  //       window.clearInterval(ticker);
  //     };
  //   }, []);

  const canvasWidth = 40000;
  const canvasHeight = 25000;

  const { canvasSvgRef } = useCanvasSizing(
    canvasWidth,
    canvasHeight,
    show,
    true
  );
  //   const fill: CSSProperties["fill"] = "hsl(202deg 32% 50%)";
  const fill: CSSProperties["fill"] = "#373737";

  const london: City = {
    name: "London",
    latLon: [51 + 30 / 60 + 26 / 3600, 0 + 7 / 60 + 39 / 3600],
  };

  const newYork: City = {
    name: "New York",
    latLon: [40 + 42 / 60 + 46 / 3600, -74 - 0 - 22 / 3600],
  };

  const beijing: City = {
    name: "Beijing",
    latLon: [39 + 54 / 60 + 26 / 3600, 116 + 23 / 60 + 22 / 3600],
  };

  const cities: City[] = [
    london,
    newYork,
    {
      name: "Tokyo",
      latLon: [35 + 41 / 60 + 12 / 3600, 139 + 46 / 60 + 40 / 3600],
    },
    {
      name: "Hawaii",
      latLon: [19 + 45 / 60 + 59 / 3600, -155 - 58 / 60 - 58 / 3600],
    },
    {
      name: "Singapore",
      latLon: [1 + 22 / 60 + 14 / 3600, 103 + 45 / 60 + 59 / 3600],
    },
    {
      name: "Hong Kong",
      latLon: [22 + 27 / 60 + 59 / 3600, 114 + 0 - 53 / 3600],
    },
  ];

  let extraPaths = toGeodesicPaths(beijing, newYork, 500).map((p) => ({
    ...p,
    stroke: "green",
    strokeWidth: 60,
  }));

  extraPaths = [
    ...extraPaths,
    ...toGeodesicPaths(newYork, london, 200).map((p) => ({
      ...p,
      stroke: "green",
      strokeWidth: 60,
    })),
  ];

  return (
    <Box
      sx={{
        width: "100vw",
        height: "100vh",
        position: "fixed",
        top: 0,
        left: 0,
        overflow: "hidden",
        backgroundColor: "#ddd",
      }}
    >
      <WorldMap
        canvasSvgRef={canvasSvgRef as any}
        canvasWidth={canvasWidth}
        canvasHeight={canvasHeight}
        fill={fill}
        paths={extraPaths}
        markers={[
          {
            lonLat: [london.latLon[1], london.latLon[0]],
            fill: "green",
            radius: 200,
            strokeWidth: 80,
            stroke: "#fff",
          },
          {
            lonLat: [newYork.latLon[1], newYork.latLon[0]],
            fill: "green",
            radius: 200,
            strokeWidth: 80,
            stroke: "#fff",
          },
          {
            lonLat: [beijing.latLon[1], beijing.latLon[0]],
            fill: "green",
            radius: 200,
            strokeWidth: 80,
            stroke: "#fff",
          },
        ]}
      />
    </Box>
  );
}
