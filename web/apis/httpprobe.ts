import {
  RawPingEvent,
  HTTPProbeEvent,
  EventObject,
  FILTERKEY_CORR_ID,
  FILTERKEY_FROM,
} from "./types";

function convertRawPingEventToEventObj(
  rawPingEv: RawPingEvent<HTTPProbeEvent> | undefined | null,
  idx: number,
): EventObject | undefined {
  const from = rawPingEv?.metadata?.from;
  const corrId = rawPingEv?.data?.correlationId;
  const date = rawPingEv?.data?.transport?.Date;
  const name = rawPingEv?.data?.transport?.Name;
  const ty = rawPingEv?.data?.transport?.Type;
  const val = rawPingEv?.data?.transport?.Value;
  const err = rawPingEv?.data?.error;
  if (!from || !corrId || !date) {
    return;
  }
  const labels = {
    [FILTERKEY_FROM]: from,
    [FILTERKEY_CORR_ID]: corrId,
  };

  let tx: number;
  try {
    const x = Date.parse(date);
    if (Number.isNaN(x) || !Number.isFinite(x)) {
      console.error("failed to parse date:", x, "from", rawPingEv);
      return undefined;
    }
    tx = x;
  } catch (e) {
    console.error("failed to parse date:", e, "from", rawPingEv);
    return undefined;
  }

  if (err) {
    return {
      id: `err-${idx}`,
      message: String(err),
      timestamp: tx || Date.now(),
      labels,
    };
  }

  const evObj: EventObject = {
    id: `${from}:${corrId}:${idx}`,
    labels: labels,
    timestamp: tx,
    annotations: ty
      ? {
          Type: ty,
        }
      : undefined,
    message: `${name ?? ""}: ${val ?? ""}`,
  };
  console.log("[dbg] generated ev:", evObj);

  return evObj;
}

export class EventAdapter extends TransformStream<unknown, EventObject> {
  constructor(private idx: number = 0) {
    super({
      transform(
        chunk: unknown,
        controller: TransformStreamDefaultController<unknown>,
      ) {
        const rawEventObj = chunk as RawPingEvent<HTTPProbeEvent>;
        if (!rawEventObj) {
          console.error("skipping nil raw ping event:", rawEventObj);
          return;
        }
        const evObj = convertRawPingEventToEventObj(rawEventObj, idx++);
        if (evObj) {
          controller.enqueue(evObj);
        } else {
          console.error("Ignore raw ping event:", rawEventObj);
        }
      },
    });
  }
}
