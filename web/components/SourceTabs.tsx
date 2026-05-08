import { Fragment } from "react";
import { Tab, Tabs, Typography, Box } from "@mui/material";

export function SourceTabs(props: {
  tabs: string[];
  active: string;
  onChange: (tab: string) => void;
}) {
  const {
    tabs: tasks,
    active: validProvider,
    onChange: setActiveProvider,
  } = props;
  return (
    <Fragment>
      <Tabs
        value={validProvider}
        onChange={(_, newValue: string) => setActiveProvider(newValue)}
        variant="scrollable"
        scrollButtons="auto"
        sx={{
          minHeight: 40,
          borderBottom: "1px solid",
          borderColor: "divider",
          "& .MuiTab-root": {
            minHeight: 40,
            textTransform: "none",
            fontWeight: 500,
          },
        }}
      >
        {tasks.map((source) => {
          return (
            <Tab
              key={source}
              value={source}
              label={
                <Box
                  sx={{
                    display: "flex",
                    alignItems: "center",
                    gap: 0.75,
                  }}
                >
                  <Typography variant="body2">{source}</Typography>
                </Box>
              }
            />
          );
        })}
      </Tabs>
    </Fragment>
  );
}
