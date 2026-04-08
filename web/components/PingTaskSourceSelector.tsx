import {
  getCurrentPingerOptions,
  NodeAttrASN,
  NodeAttrISP,
} from "@/apis/globalping";
import { PendingTask } from "@/apis/types";
import { SourceOption, SourcesSelector } from "@/components/sourceselector";
import { Dispatch, SetStateAction } from "react";

const fakeSources: SourceOption[] = [
  {
    key: "us-nyc-01",
    label: "us-nyc1",
    iso3166alpha2: "US",
    cityName: "New York",
  },
  {
    key: "us-dal-01",
    label: "us-dal1",
  },
  {
    key: "somewhere",
    label: "Somewhere",
    iso3166alpha2: "",
    cityName: "",
  },
  {
    key: "cn-pek-01",
    label: "cn-pek1",
    iso3166alpha2: "CN",
    cityName: "Beijing",
  },
];

export function PingTaskSourceSelector(props: {
  pendingTask: PendingTask;
  setPendingTask: Dispatch<SetStateAction<PendingTask>>;
}) {
  const { pendingTask, setPendingTask } = props;

  return (
    <SourcesSelector
      value={pendingTask.sources
        .map((s) => s.trim())
        .filter((s) => s.length > 0)}
      onChange={(value) =>
        setPendingTask((prev) => ({
          ...prev,
          sources: value.map((s) => s.trim()).filter((s) => !!s),
        }))
      }
      getOptions={() => {
        // return Promise.resolve(fakeSources);

        let filter: Record<string, string> | undefined = undefined;
        if (!!pendingTask.useUDP) {
          filter = { ...(filter || {}), SupportUDP: "true" };
        }
        if (!!pendingTask.pmtu) {
          filter = { ...(filter || {}), SupportPMTU: "true" };
        }
        if (pendingTask.type === "tcpping") {
          filter = { ...(filter || {}), SupportTCP: "true" };
        }
        if (pendingTask.type === "dns") {
          filter = {
            ...(filter || {}),
            CapabilityDNSProbe: "true",
          };
        }
        if (pendingTask.type === "http") {
          filter = {
            ...(filter || {}),
            CapabilityHTTPProbe: "true",
          };
        }

        return getCurrentPingerOptions(filter).then((nodes) => {
          return nodes.map((node) => ({
            key: node.node_name ?? "",
            label: node.node_name ?? "",
            iso3166alpha2: node.attributes?.CountryCode,
            cityName: node.attributes?.CityName,
            asn: node.attributes?.[NodeAttrASN],
            isp: node.attributes?.[NodeAttrISP],
          }));
        });
        // return new Promise((res) => {
        //   window.setTimeout(() => res(fakeSources), 2000);
        // });
      }}
    />
  );
}
