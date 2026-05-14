#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
EXEC_START_PATH="${PROJECT_ROOT}/scripts/fetch-mmdb-files.sh"
UNIT_NAME="fetch-mmdb-files"

# --- Render Jinja2 templates into concrete unit files -----------------------
python3 - <<'PYEOF' "$SCRIPT_DIR" "$EXEC_START_PATH"
import jinja2, os, sys

script_dir   = sys.argv[1]
exec_start   = sys.argv[2]

env = jinja2.Environment(
    loader=jinja2.FileSystemLoader(script_dir),
    keep_trailing_newline=True,
)

for tmpl_name in ("fetch-mmdb-files.service.j2", "fetch-mmdb-files.timer.j2"):
    out_name = tmpl_name[:-3]                       # strip ".j2"
    tmpl     = env.get_template(tmpl_name)
    rendered = tmpl.render(exec_start_path=exec_start)
    with open(os.path.join(script_dir, out_name), "w") as fh:
        fh.write(rendered)
    print(f"  rendered {out_name}")

print("Templates rendered successfully.")
PYEOF

# --- Install the rendered unit files -----------------------------------------
systemctl link "${SCRIPT_DIR}/${UNIT_NAME}.service"
systemctl link "${SCRIPT_DIR}/${UNIT_NAME}.timer"
systemctl enable --now "${UNIT_NAME}.timer"

echo "Timer '${UNIT_NAME}' installed and started."
systemctl status "${UNIT_NAME}.timer" --no-pager
