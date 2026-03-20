export const wordSize = 4;

export function createStreamFromDataChannel(channel: RTCDataChannel) {
  return new ReadableStream({
    start(controller) {
      channel.onmessage = (event) => {
        controller.enqueue(event.data);
      };
    },
  });
}

function mergeArrayBuffers(chunks: ArrayBuffer[], totalSize: number) {
  const mergedBuffer = new ArrayBuffer(totalSize);
  const dv = new DataView(mergedBuffer);
  let offset = 0;
  for (const chunk of chunks) {
    const u8s = new Uint8Array(chunk);
    u8s.forEach((octet, i) => dv.setUint8(offset + i, octet));
    offset += chunk.byteLength;
  }
  return mergedBuffer;
}

// parse a network stream in network byte order (big endian) into a stream of 32-bit words
export function newUint32StreamParser() {
  let feedBackParseRef: {
    chunks: ArrayBuffer[];
    totalSize: number;
  } = {
    chunks: [],
    totalSize: 0,
  };

  const doConsume = (controller: TransformStreamDefaultController<number>) => {
    if (feedBackParseRef.totalSize >= wordSize) {
      const nWords = Math.floor(feedBackParseRef.totalSize / wordSize);
      const restSize = feedBackParseRef.totalSize % wordSize;
      const mergedBuffer = mergeArrayBuffers(
        feedBackParseRef.chunks,
        feedBackParseRef.totalSize,
      );
      const dv = new DataView(mergedBuffer);
      for (let i = 0; i < nWords; i++) {
        const offset = i * wordSize;
        const word = dv.getUint32(offset, false);
        controller.enqueue(word);
      }
      feedBackParseRef.chunks = [];
      feedBackParseRef.totalSize = 0;
      if (restSize > 0) {
        const restChunk = mergedBuffer.slice(
          mergedBuffer.byteLength - restSize,
        );
        feedBackParseRef.chunks.push(restChunk);
        feedBackParseRef.totalSize += restSize;
      }
    }
  };
  return new TransformStream({
    transform(chunk, controller) {
      if (!(chunk instanceof ArrayBuffer)) {
        controller.error(new Error("chunk has unknown binary type"));
        return;
      }

      feedBackParseRef.chunks.push(chunk);
      feedBackParseRef.totalSize += chunk.byteLength;
      doConsume(controller);
    },
    flush(controller) {
      doConsume(controller);
    },
  });
}
