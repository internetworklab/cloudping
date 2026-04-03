import { RawPingEvent, RawPingEventData, ExactLocation } from "./types";

// PingEvent represents a single ping/traceroute event
export class TraceEvent {
  constructor(
    public readonly Err: string,
    public readonly Seq: number,
    public readonly RTTMs: number,
    public readonly Peer: string,
    public readonly PeerRDNS: string,
    public readonly IPPacketSize: number,
    public readonly Timeout: boolean,
    public readonly ASN: string,
    public readonly ISP: string,
    public readonly CountryAlpha2: string,
    public readonly City: string,
    public readonly ExactLocation: ExactLocation | null,
    public readonly TTL: number,
    public readonly OriginTTL: number,
    public readonly LastHop: boolean,
    public readonly PMTU: number | undefined,
  ) {}

  static fromError(err: string): TraceEvent {
    return new TraceEvent(
      err,
      0,
      0,
      "",
      "",
      0,
      false,
      "",
      "",
      "",
      "",
      null,
      0,
      0,
      false,
      undefined,
    );
  }

  static fromRaw(raw: RawPingEvent<RawPingEventData>): TraceEvent | null {
    // Check for upstream error fields, mirroring Go's GetEvents logic:
    // pingEVObj.Err (string pointer) and pingEVObj.Error (error interface)
    if (raw.err) {
      return TraceEvent.fromError(raw.err);
    }
    if (raw.error) {
      return TraceEvent.fromError(raw.error);
    }

    // Check metadata presence and target, matching Go's skip logic
    if (!raw.metadata || !raw.metadata.target) {
      return null;
    }

    const data = raw.data;
    if (!data) {
      // Mirrors Go's convertPingEventToBotEvent which returns an error
      // when pingEV.Data is nil
      return TraceEvent.fromError("ping event data is nil");
    }

    const seq = data.Seq ?? 0;
    const rttMs =
      data.RTTMilliSecs && data.RTTMilliSecs.length > 0
        ? data.RTTMilliSecs[0]
        : 0;
    const timeout = !data.ReceivedAt || data.ReceivedAt.length === 0;
    const originTTL = data.TTL ?? 0;

    let peer = "";
    let peerRDNS = "";
    let ipPacketSize = 0;
    let ttl = 0;
    let lastHop = false;
    let asn = "";
    let isp = "";
    let city = "";
    let countryAlpha2 = "";
    let exactLocation: ExactLocation | null = null;
    let pmtu: number | undefined = undefined;

    if (data.Raw && data.Raw.length > 0) {
      const rawReply = data.Raw[data.Raw.length - 1];

      peer = rawReply.Peer ?? "";

      if (rawReply.PeerRDNS && rawReply.PeerRDNS.length > 0) {
        peerRDNS = rawReply.PeerRDNS[0];
      }

      ipPacketSize = rawReply.Size ?? 0;
      ttl = rawReply.TTL ?? 0;
      lastHop = rawReply.LastHop ?? false;
      pmtu = rawReply.SetMTUTo ?? undefined;

      // Extract IP info fields
      if (rawReply.PeerIPInfo) {
        const ipInfo = rawReply.PeerIPInfo;
        asn = ipInfo.ASN ?? "";
        isp = ipInfo.ISP ?? "";
        city = ipInfo.City ?? "";
        countryAlpha2 = ipInfo.ISO3166Alpha2 ?? "";
        if (ipInfo.Exact) {
          exactLocation = ipInfo.Exact;
        }
      }

      // Fallback to direct peer fields if PeerIPInfo is not available
      if (asn === "" && rawReply.PeerASN != null) {
        asn = rawReply.PeerASN;
      }
      if (isp === "" && rawReply.PeerISP != null) {
        isp = rawReply.PeerISP;
      }
      if (exactLocation === null && rawReply.PeerExactLocation != null) {
        exactLocation = rawReply.PeerExactLocation;
      }
    }

    return new TraceEvent(
      "",
      seq,
      rttMs,
      peer,
      peerRDNS,
      ipPacketSize,
      timeout,
      asn,
      isp,
      countryAlpha2,
      city,
      exactLocation,
      ttl,
      originTTL,
      lastHop,
      pmtu,
    );
  }
}

// PeerStats holds statistics and events for a single peer (IP address) at a hop.
// Immutable: WriteEvent returns a new PeerStats.
export class PeerStats {
  constructor(
    public readonly Peer: string = "",
    public readonly PeerRDNS: string = "",
    public readonly ASN: string = "",
    public readonly ISP: string = "",
    public readonly City: string = "",
    public readonly CountryAlpha2: string = "",
    public readonly Events: readonly TraceEvent[] = [],
    public readonly ReceivedCount: number = 0,
    public readonly LossCount: number = 0,
    public readonly MinRTT: number = -1,
    public readonly MaxRTT: number = -1,
    public readonly TotalRTT: number = 0,
    public readonly PMTU: number | undefined = undefined,
  ) {}

  // AvgRTT returns the average RTT for this peer
  AvgRTT(): number {
    if (this.ReceivedCount === 0) {
      return 0;
    }
    return Math.floor(this.TotalRTT / this.ReceivedCount);
  }

  // WriteEvent returns a new PeerStats with the event applied
  WriteEvent(ev: TraceEvent): PeerStats {
    const newEvents = [...this.Events, ev].sort((a, b) => a.Seq - b.Seq);

    if (ev.Timeout) {
      return new PeerStats(
        this.Peer || ev.Peer,
        this.PeerRDNS,
        this.ASN,
        this.ISP,
        this.City,
        this.CountryAlpha2,
        newEvents,
        this.ReceivedCount,
        this.LossCount + 1,
        this.MinRTT,
        this.MaxRTT,
        this.TotalRTT,
        this.PMTU,
      );
    }

    return new PeerStats(
      this.Peer || ev.Peer,
      ev.PeerRDNS !== "" ? ev.PeerRDNS : this.PeerRDNS,
      ev.ASN !== "" ? ev.ASN : this.ASN,
      ev.ISP !== "" ? ev.ISP : this.ISP,
      ev.City !== "" ? ev.City : this.City,
      ev.CountryAlpha2 !== "" ? ev.CountryAlpha2 : this.CountryAlpha2,
      newEvents,
      this.ReceivedCount + 1,
      this.LossCount,
      this.MinRTT === -1 || ev.RTTMs < this.MinRTT ? ev.RTTMs : this.MinRTT,
      this.MaxRTT === -1 || ev.RTTMs > this.MaxRTT ? ev.RTTMs : this.MaxRTT,
      this.TotalRTT + ev.RTTMs,
      ev.PMTU ?? this.PMTU,
    );
  }
}

// HopGroup holds statistics for a single hop (TTL level).
// Immutable: WriteEvent returns a new HopGroup.
export class HopGroup {
  constructor(
    public readonly TTL: number = 0,
    public readonly Peers: ReadonlyMap<string, PeerStats> = new Map(),
    public readonly PeerOrder: readonly string[] = [],
  ) {}

  // WriteEvent returns a new HopGroup with the event applied
  WriteEvent(ev: TraceEvent): HopGroup {
    let peerKey = ev.Peer;
    if (ev.Timeout || peerKey === "") {
      peerKey = "*";
    }

    const existingPeer = this.Peers.get(peerKey);
    const newPeer = existingPeer
      ? existingPeer.WriteEvent(ev)
      : new PeerStats().WriteEvent(ev);

    const newPeers = new Map(this.Peers);
    newPeers.set(peerKey, newPeer);

    // Sort PeerOrder by max seq (descending) - peer with latest packets first
    const newPeerOrder = [...newPeers.keys()].sort((a, b) => {
      const peerA = newPeers.get(a)!;
      const peerB = newPeers.get(b)!;
      if (peerA.Events.length === 0) return 1;
      if (peerB.Events.length === 0) return -1;
      const maxSeqA = peerA.Events[peerA.Events.length - 1].Seq;
      const maxSeqB = peerB.Events[peerB.Events.length - 1].Seq;
      return maxSeqB - maxSeqA;
    });

    return new HopGroup(this.TTL, newPeers, newPeerOrder);
  }
}

// TraceStats holds the complete traceroute statistics.
// Immutable: WriteEvent returns a new TraceStats.
export class TraceStats {
  constructor(
    public readonly Hops: ReadonlyMap<number, HopGroup> = new Map(),
    public readonly HopOrder: readonly number[] = [],
  ) {}

  // WriteEvent returns a new TraceStats with the event applied
  WriteEvent(ev: TraceEvent): TraceStats {
    let hopTTL = ev.OriginTTL;
    if (hopTTL <= 0) {
      // Fallback for events without OriginTTL (shouldn't happen in traceroute)
      hopTTL = 1;
    }

    const existingHop = this.Hops.get(hopTTL);
    const newHop = existingHop
      ? existingHop.WriteEvent(ev)
      : new HopGroup(hopTTL).WriteEvent(ev);

    const newHops = new Map(this.Hops);
    newHops.set(hopTTL, newHop);

    let newHopOrder = existingHop
      ? [...this.HopOrder]
      : [...this.HopOrder, hopTTL].sort((a, b) => a - b);

    // If this is the last hop, remove all higher hops
    if (ev.LastHop) {
      newHopOrder = newHopOrder.filter((ttl) => ttl <= hopTTL);
      const filteredHops = new Map<number, HopGroup>();
      for (const [ttl, hop] of newHops) {
        if (ttl <= hopTTL) {
          filteredHops.set(ttl, hop);
        }
      }
      return new TraceStats(filteredHops, newHopOrder);
    }

    return new TraceStats(newHops, newHopOrder);
  }

  // Design:
  //
  // ```
  // Hop  Peer                      RTTs (Last Min/Avg/Max)  Stats (Rx/Tx/Loss)
  //      (IP address)              ASN Network              City,Country
  //
  // 1.   homelab.local             1ms 1ms/2ms/3ms          2/3/33%
  //      (192.168.1.1)[PMTU=1492]  RFC1918
  //
  // 2.   a.example.com             10ms 10ms/10ms/10ms      3/3/0%
  //      (17.18.19.20)             AS12345 Example LLC      HongKong,HK
  //
  // 3.   [TIMEOUT]
  //      (*)
  //
  // 4.   google.com                100ms 100ms/100ms/100ms  1/1/0%
  // ```
  //
  // Note:
  //
  // 1. If RDNS is empty string, use IP address as RDNS
  // 2. A spacer (of one row height) is between each hop

  // ToTable converts the traceroute statistics to a Table struct
  ToTable(): Table {
    const header: Row[] = [
      Row.fromCells([
        "Hop",
        "Peer",
        "RTTs (Last Min/Avg/Max)",
        "Stats (Rx/Tx/Loss)",
      ]),
      Row.fromCells(["", "(IP address)", "ASN Network", "City,Country"]),
    ];
    const rows: Row[] = [];

    if (this.HopOrder.length === 0) {
      return new Table(header, rows);
    }

    for (let hopIdx = 0; hopIdx < this.HopOrder.length; hopIdx++) {
      const hopTTL = this.HopOrder[hopIdx];

      // Add blank row between hops
      if (hopIdx > 0) {
        rows.push(Row.spacerRow());
      }

      const hop = this.Hops.get(hopTTL);
      if (!hop) {
        continue;
      }

      let isFirstPeer = true;
      for (const peerKey of hop.PeerOrder) {
        const peerStats = hop.Peers.get(peerKey);
        if (!peerStats) {
          continue;
        }

        // Row 1: Hop number, Peer name, RTT stats, Packet stats
        let hopCell = "";
        if (isFirstPeer) {
          hopCell = `${hopTTL}.`;
          isFirstPeer = false;
        }

        // Peer name: [TIMEOUT] for timed out peers, RDNS or IP otherwise
        const isTimeout =
          peerStats.ReceivedCount === 0 && peerStats.LossCount > 0;
        let peerName = "";
        if (isTimeout) {
          peerName = "[TIMEOUT]";
        } else {
          peerName = peerStats.PeerRDNS;
          if (peerName === "") {
            peerName = peerStats.Peer;
          }
          if (peerName === "" || peerName === "*") {
            peerName = "*";
          }
        }

        // RTT stats: last_rtt min/avg/max
        let rttCell = "";
        if (isTimeout) {
          // No RTT data for timeout peers
        } else if (peerStats.ReceivedCount > 0 && peerStats.Events.length > 0) {
          let lastRTT = 0;
          for (let i = peerStats.Events.length - 1; i >= 0; i--) {
            if (!peerStats.Events[i].Timeout) {
              lastRTT = peerStats.Events[i].RTTMs;
              break;
            }
          }
          const avgRTT = Math.floor(
            peerStats.TotalRTT / peerStats.ReceivedCount,
          );
          rttCell = `${lastRTT}ms ${peerStats.MinRTT}ms/${avgRTT}ms/${peerStats.MaxRTT}ms`;
        } else {
          rttCell = "* */*/*";
        }

        // Packet stats: received/total/loss%
        let statsCell = "";
        if (!isTimeout) {
          const totalPkts = peerStats.ReceivedCount + peerStats.LossCount;
          let lossPercent = 0;
          if (totalPkts > 0) {
            lossPercent = (peerStats.LossCount / totalPkts) * 100;
          }
          statsCell = `${peerStats.ReceivedCount}/${totalPkts}/${lossPercent.toFixed(0)}%`;
        }

        rows.push(Row.fromCells([hopCell, peerName, rttCell, statsCell]));

        // Row 2: IP address, ASN/ISP, Location
        let ipCell = "(*)";
        if (
          peerStats.Peer !== "" &&
          peerStats.Peer !== "*" &&
          peerStats.ReceivedCount > 0
        ) {
          ipCell = `(${peerStats.Peer})`;
        }

        if (peerStats.PMTU !== undefined && peerStats.PMTU !== null) {
          ipCell += `[PMTU=${peerStats.PMTU}]`;
        }

        // ASN/ISP info (column 3, row 2)
        let asnIspCell = "";
        if (peerStats.ASN !== "" || peerStats.ISP !== "") {
          if (peerStats.ASN !== "") {
            asnIspCell = peerStats.ASN;
          }
          if (peerStats.ISP !== "") {
            if (asnIspCell !== "") {
              asnIspCell += " " + peerStats.ISP;
            } else {
              asnIspCell = peerStats.ISP;
            }
          }
        }

        // Location info (column 4, row 2)
        let locationCell = "";
        if (peerStats.City !== "" || peerStats.CountryAlpha2 !== "") {
          if (peerStats.City !== "") {
            locationCell = peerStats.City;
          }
          if (peerStats.CountryAlpha2 !== "") {
            if (locationCell !== "") {
              locationCell += "," + peerStats.CountryAlpha2;
            } else {
              locationCell = peerStats.CountryAlpha2;
            }
          }
        }

        rows.push(Row.fromCells(["", ipCell, asnIspCell, locationCell]));
      }
    }

    return new Table(header, rows);
  }
}

// Row represents a row in a table
export class Row {
  constructor(
    public cells: string[] = [],
    public spacer?: boolean,
  ) {}

  static spacerRow(): Row {
    return new Row([], true);
  }

  static fromCells(cells: string[]): Row {
    return new Row(cells, false);
  }
}

// Table represents a table of rows
export class Table {
  constructor(
    public header: Row[],
    public rows: Row[],
  ) {}
}

// getExampleTable returns a sample traceroute table for demonstration purposes.
// It illustrates the expected table layout described in the design notes above.
export function getExampleTable(): Table {
  const header: Row[] = [
    Row.fromCells([
      "Hop",
      "Peer",
      "RTTs (Last Min/Avg/Max)",
      "Stats (Rx/Tx/Loss)",
    ]),
    Row.fromCells(["", "(IP address)", "ASN Network", "City,Country"]),
  ];

  const rows: Row[] = [
    Row.fromCells(["1.", "homelab.local", "1ms 1ms/2ms/3ms", "2/3/33%"]),
    Row.fromCells(["", "(192.168.1.1)[PMTU=1492]", "RFC1918", ""]),
    Row.spacerRow(),
    Row.fromCells(["2.", "a.example.com", "10ms 10ms/10ms/10ms", "3/3/0%"]),
    Row.fromCells(["", "(17.18.19.20)", "AS12345 Example LLC", "HongKong,HK"]),
    Row.spacerRow(),
    Row.fromCells(["3.", "[TIMEOUT]", "", ""]),
    Row.fromCells(["", "(*)", "", ""]),
    Row.spacerRow(),
    Row.fromCells(["4.", "google.com", "100ms 100ms/100ms/100ms", "1/1/0%"]),
  ];

  return new Table(header, rows);
}
