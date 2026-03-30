import { IPAddressFamily, IPAddrLike } from "@/utls/router";
import { isValid, parse } from "ipaddr.js";

export class IPAddr implements IPAddrLike {
  constructor(
    public data: Uint8Array,
    public family: IPAddressFamily,
  ) {}

  static fromString(addr: string): IPAddr | undefined {
    if (isValid(addr)) {
      const ipObj = parse(addr);
      return new IPAddr(
        new Uint8Array(ipObj.toByteArray()),
        ipObj.kind() === "ipv4" ? IPAddressFamily.IPv4 : IPAddressFamily.IPv6,
      );
    }
    return undefined;
  }

  getFamily(): IPAddressFamily {
    return this.family;
  }

  getBytes(): Uint8Array {
    return this.data;
  }

  getMaskedValue(bitOffset: number, nbits: number): bigint {
    if (nbits === 0) {
      return BigInt(0);
    }

    // nbits hsb are 0s, the rest are all 1s
    const revmask = (nbits: number): bigint =>
      (BigInt(1) << BigInt(8 - nbits)) - BigInt(1);

    // nbits hsb are 1s, the rest are all 0s
    const mask = (nbits: number) =>
      BigInt(0xff) - ((BigInt(1) << BigInt(8 - nbits)) - BigInt(1));
    const byteOffset = Math.floor(bitOffset / 8);
    bitOffset = bitOffset % 8;
    let value = BigInt(0);
    for (let i = 0; i < this.data.byteLength; i++) {
      if (i < byteOffset) {
        continue;
      } else if (i === byteOffset) {
        value = BigInt(this.data[i]) & revmask(bitOffset);
        const bitsRead = Math.min(8 - bitOffset, nbits);
        value = value & mask(bitOffset + bitsRead);
        value = value >> BigInt(8 - bitOffset - bitsRead);
        nbits -= bitsRead;
      } else {
        if (nbits > 0) {
          const bitsRead = Math.min(8, nbits);
          value = value << BigInt(bitsRead);
          value =
            value |
            ((BigInt(this.data[i]) & mask(bitsRead)) >> BigInt(8 - bitsRead));
          nbits -= bitsRead;
        }
      }
    }

    return value;
  }

  toMask(prefixLen: number): IPAddrLike {
    const bytes: number[] = [];
    for (let i = 0; i < this.data.byteLength; i++) {
      const bitsOffset = i * 8;
      if (bitsOffset >= prefixLen) {
        bytes.push(0);
      } else {
        const nbits = Math.min(8, prefixLen - bitsOffset);
        const mask = 0xff - ((1 << (8 - nbits)) - 1);
        bytes.push(this.data[i] & mask);
      }
    }
    return new IPAddr(new Uint8Array(bytes), this.family);
  }
}
