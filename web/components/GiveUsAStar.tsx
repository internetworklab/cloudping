"use client";

import { getRepo } from "@/apis/getrepo";
import { Link, Tooltip } from "@mui/material";
import { useQuery } from "@tanstack/react-query";
import StarBorderIcon from "@mui/icons-material/StarBorder";

export function GiveUsAStar(props: { repoOwner: string; repoName: string }) {
  const { repoOwner, repoName } = props;
  const { data: repoDetails } = useQuery({
    queryKey: [
      "stargazers_count",
      "repooOwner",
      repoOwner,
      "repoName",
      repoName,
    ],
    queryFn: () =>
      repoOwner && repoName ? getRepo(repoOwner, repoName) : undefined,
  });
  return (
    <Tooltip title="Give us a star!">
      <Link
        underline="hover"
        href={repoDetails?.html_url ?? ""}
        target="_blank"
        variant="caption"
        sx={{ display: "flex", alignItems: "center", gap: 0.5 }}
      >
        <StarBorderIcon />
        {repoDetails?.stargazers_count ?? 0}
      </Link>
    </Tooltip>
  );
}
