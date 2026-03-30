import { useQuery } from "@tanstack/react-query";
import { isValidCIDR, parseCIDR } from "ipaddr.js";
import { IPAddr } from "@/utls/ipaddr";
import { IPAddressFamily, Route, buildTable, lookup } from "@/utls/router";
import {
  NetworkDescriptor,
  DomainDescriptor,
  domainDescriptorLookup,
} from "@/apis/nwdesc";

export function useAddressClassify(input: string) {
  const parsed = IPAddr.fromString(input);

  const { data, isLoading: isRoutesLoading } = useQuery({
    queryKey: ["network-descriptors"],
    queryFn: async () => {
      const res = await fetch("/networkdescriptor.json");
      const descriptors: NetworkDescriptor[] = await res.json();
      const routes: Route[] = [];
      for (const desc of descriptors) {
        if (!isValidCIDR(desc.prefix)) {
          continue;
        }
        const [ipObj, prefixLength] = parseCIDR(desc.prefix);
        routes.push({
          networkAddr: new IPAddr(
            new Uint8Array(ipObj.toByteArray()),
            ipObj.kind() === "ipv4"
              ? IPAddressFamily.IPv4
              : IPAddressFamily.IPv6,
          ),
          prefixLength,
          value: desc,
        });
      }
      return { routes, table: buildTable(routes) };
    },
  });

  const { data: domainData, isLoading: isDomainLoading } = useQuery({
    queryKey: ["domain-descriptors"],
    queryFn: async () => {
      const res = await fetch("/domaindescriptor.json");
      const descriptors: DomainDescriptor[] = await res.json();
      return { descriptors };
    },
  });

  const lookupResult =
    parsed && data?.table ? lookup(data.table, parsed) : undefined;
  const matchedRouteIds = new Set<string>(
    lookupResult?.routes?.map((r) => (r.value as NetworkDescriptor).id) ?? [],
  );

  const matchedDomainIds = new Set<string>(
    !parsed && input && domainData?.descriptors
      ? domainDescriptorLookup(domainData.descriptors, input).map((d) => d.id)
      : [],
  );

  return {
    parsed,
    routes: data?.routes ?? [],
    domainDescriptors: domainData?.descriptors ?? [],
    matchedRouteIds,
    matchedDomainIds,
    isRoutesLoading,
    isDomainLoading,
  };
}
