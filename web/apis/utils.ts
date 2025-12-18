import { PingSample } from "./globalping";

export function streamFromSamples(
  samples: PingSample[]
): ReadableStream<PingSample> {
  const baseDelayMs = 300;
  const jitterMs = 125;

  let closed = false;
  let timeoutIds: Array<ReturnType<typeof setTimeout>> = [];

  const clearAllTimeouts = () => {
    for (const tid of timeoutIds) {
      globalThis.clearTimeout(tid);
    }
    timeoutIds = [];
  };

  return new ReadableStream<PingSample>({
    start(controller) {
      if (samples.length === 0) {
        closed = true;
        controller.close();
        return;
      }

      timeoutIds = samples.map((sample, idx) => {
        const delayMs =
          baseDelayMs * (idx + 1) + Math.floor(Math.random() * jitterMs);
        return globalThis.setTimeout(() => {
          if (closed) {
            return;
          }
          controller.enqueue(sample);
          if (idx === samples.length - 1) {
            closed = true;
            controller.close();
          }
        }, delayMs);
      });
    },
    cancel() {
      closed = true;
      clearAllTimeouts();
    },
  });
}
