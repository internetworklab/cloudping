"use client";

import { Autocomplete, Box, Chip, TextField } from "@mui/material";
import { useState, Fragment } from "react";
import { CircularProgress } from "@mui/material";
import {
  MRTEntryProvidersLister,
  MRTEntriesProvider,
  ProviderStatus,
} from "../apis/mrtProviders";

// generated from deepseek
function getFlagEmoji(countryCode: string): string {
  // 确保是大写字母
  const code = countryCode.toUpperCase();

  // 将每个字母转换为对应的区域指示符号Unicode字符
  // 区域指示符号A的Unicode是U+1F1E6
  // 使用codePointAt获取字母的Unicode码点
  const firstCP = code.codePointAt(0);
  const secondCP = code.codePointAt(1);
  if (firstCP === undefined || secondCP === undefined) {
    return "";
  }

  const firstChar = firstCP - 0x41 + 0x1f1e6;
  const secondChar = secondCP - 0x41 + 0x1f1e6;

  // 使用String.fromCodePoint创建emoji
  return String.fromCodePoint(firstChar, secondChar);
}

export type SourceOption = {
  key: string;
  label: string;
  iso3166alpha2?: string;
  cityName?: string;
  asn?: string;
  isp?: string;
};

function getOptionLabel(opt: SourceOption): string {
  const basic = opt.label.toUpperCase();
  if (opt.iso3166alpha2 && opt.iso3166alpha2.length === 2) {
    return `${getFlagEmoji(opt.iso3166alpha2)} ${basic}`;
  }

  return basic;
}

export function SourcesSelector(props: {
  value: string[];
  onChange: (value: string[]) => void;
  getOptions: () => Promise<SourceOption[]>;
}) {
  const { onChange, getOptions } = props;
  const [options, setOptions] = useState<SourceOption[]>([]);
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [isOpen, setIsOpen] = useState<boolean>(false);
  const valSet = new Set(props.value);
  const optionsSelected = options.filter((opt) => valSet.has(opt.key));

  return (
    <Autocomplete
      fullWidth
      value={optionsSelected}
      open={isOpen}
      onClose={() => setIsOpen(false)}
      getOptionKey={(opt) => opt.key}
      getOptionLabel={getOptionLabel}
      onOpen={() => {
        setIsOpen(true);
        setIsLoading(true);
        getOptions()
          .then((options) => setOptions(options))
          .finally(() => setIsLoading(false));
      }}
      onChange={(_, value) => onChange(value.map((v) => v.key))}
      renderOption={(props, option, _, ownerSt) => {
        const { key, ...optionProps } = props;

        let cityLoc: string[] = [];
        if (option.cityName) {
          cityLoc.push(option.cityName);
        }
        if (option.iso3166alpha2) {
          cityLoc.push(option.iso3166alpha2);
        }
        cityLoc = cityLoc.filter((v) => !!v);

        const asnIsp: string[] = [];
        if (option.asn) {
          asnIsp.push(option.asn);
        }
        if (option.isp) {
          asnIsp.push(option.isp);
        }

        return (
          <Box key={key} component="li" {...optionProps}>
            {ownerSt.getOptionLabel(option)}
            {cityLoc.length > 0 && (
              <Box fontSize={"small"} sx={{ marginLeft: 1 }} component={"span"}>
                {cityLoc.join(", ")}
              </Box>
            )}
            {asnIsp.length > 0 && (
              <Box fontSize={"small"} sx={{ marginLeft: 1 }} component={"span"}>
                {asnIsp.join(" ")}
              </Box>
            )}
          </Box>
        );
      }}
      multiple
      options={options}
      defaultValue={[]}
      loading={isLoading}
      loadingText={"Loading..."}
      renderInput={(params) => (
        <TextField
          {...params}
          variant="standard"
          label="Sources"
          placeholder={
            optionsSelected.length > 0
              ? ""
              : "Hint: multiple items can be selected at a time"
          }
          slotProps={{
            input: {
              ...params.InputProps,
              endAdornment: (
                <Fragment>
                  {isLoading ? (
                    <CircularProgress color="inherit" size={20} />
                  ) : null}
                  {params.InputProps.endAdornment}
                </Fragment>
              ),
            },
          }}
        />
      )}
      disableCloseOnSelect
    />
  );
}

function getProviderStatusLabel(status: ProviderStatus): string {
  switch (status) {
    case ProviderStatus.Provisioning:
      return "Provisioning";
    case ProviderStatus.Ready:
      return "Ready";
    default:
      return status;
  }
}

export function MRTSourceSelector(props: {
  value: string[];
  onChange: (value: string[]) => void;
  lister: MRTEntryProvidersLister;
}) {
  const { onChange, lister } = props;
  const [options, setOptions] = useState<MRTEntriesProvider[]>([]);
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [isOpen, setIsOpen] = useState<boolean>(false);
  const valSet = new Set(props.value);
  const optionsSelected = options.filter((opt) => valSet.has(opt.Name));

  return (
    <Autocomplete
      fullWidth
      value={optionsSelected}
      open={isOpen}
      onClose={() => setIsOpen(false)}
      getOptionKey={(opt) => opt.Name}
      getOptionLabel={(opt) => opt.Name}
      onOpen={() => {
        setIsOpen(true);
        setIsLoading(true);
        lister
          .getMRTEntriesProviders()
          .then((res) => {
            if (res.data) setOptions(res.data);
          })
          .finally(() => setIsLoading(false));
      }}
      onChange={(_, value) => onChange(value.map((v) => v.Name))}
      renderOption={(props, option) => {
        const { key, ...optionProps } = props;

        return (
          <Box key={key} component="li" {...optionProps}>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
              <span>{option.Name}</span>
              <Chip
                label={getProviderStatusLabel(option.Status)}
                size="small"
                color={
                  option.Status === ProviderStatus.Ready ? "success" : "warning"
                }
                variant="outlined"
              />
              <Box
                fontSize="small"
                sx={{ marginLeft: "auto", color: "text.secondary" }}
                component="span"
              >
                {"Updated: " + option.LastModified}
              </Box>
            </Box>
          </Box>
        );
      }}
      multiple
      options={options}
      defaultValue={[]}
      loading={isLoading}
      loadingText={"Loading..."}
      renderInput={(params) => (
        <TextField
          {...params}
          variant="standard"
          label="Sources"
          placeholder={
            optionsSelected.length > 0
              ? ""
              : "Hint: multiple items can be selected at a time"
          }
          slotProps={{
            input: {
              ...params.InputProps,
              endAdornment: (
                <Fragment>
                  {isLoading ? (
                    <CircularProgress color="inherit" size={20} />
                  ) : null}
                  {params.InputProps.endAdornment}
                </Fragment>
              ),
            },
          }}
        />
      )}
      disableCloseOnSelect
    />
  );
}
