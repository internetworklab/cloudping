#!/usr/bin/env python3
import json
import ipaddress

# Load data
with open('web/public/example_mrt_entries.json') as f:
    data = json.load(f)

# RIR classification by first octet for IPv4, and by prefix for IPv6
# Using known IANA allocations (simplified)

RIPE_NCC_V4 = {2, 5, 31, 37, 46, 51, 53, 57, 62, 77, 78, 79, 80, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95, 141, 145, 151, 176, 185, 188, 193, 194, 195, 212, 213}
ARIN_V4 = {3, 6, 7, 8, 9, 11, 12, 13, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 28, 29, 30, 32, 33, 34, 35, 38, 40, 44, 45, 47, 48, 50, 52, 54, 55, 56, 63, 64, 65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 96, 97, 98, 99, 100, 104, 107, 108, 128, 129, 130, 131, 132, 133, 134, 135, 136, 137, 138, 139, 140, 142, 143, 144, 146, 147, 148, 149, 150, 152, 155, 156, 157, 158, 159, 160, 161, 162, 163, 164, 168, 169, 170, 172, 173, 174, 184, 192, 196, 198, 199, 204, 205, 206, 207, 208, 209, 210, 214, 215, 216, 217}
AFRINIC_V4 = {41, 102, 105, 154, 165, 197}
APNIC_V4 = {1, 14, 27, 36, 39, 42, 43, 49, 58, 59, 60, 61, 101, 103, 106, 110, 111, 112, 113, 114, 115, 116, 117, 118, 119, 120, 121, 122, 123, 124, 125, 126, 153, 171, 175, 180, 182, 183, 202, 203, 211, 218, 219, 220, 221, 222, 223}
LACNIC_V4 = {177, 179, 181, 186, 187, 189, 190, 191, 200, 201}

def get_rir(entry):
    prefix = entry['Data']['Prefix']
    try:
        network = ipaddress.ip_network(prefix, strict=False)
        if isinstance(network, ipaddress.IPv4Network):
            first_octet = int(str(network.network_address).split('.')[0])
            if first_octet in RIPE_NCC_V4:
                return 'ripencc'
            elif first_octet in ARIN_V4:
                return 'arin'
            elif first_octet in AFRINIC_V4:
                return 'afrinic'
            elif first_octet in APNIC_V4:
                return 'apnic'
            elif first_octet in LACNIC_V4:
                return 'lacnic'
            else:
                # Private/reserved ranges: 10, 172, 192.168 -> distribute evenly
                return 'unknown'
        else:
            # IPv6: classify by first hex digit group
            first = str(network.network_address).split(':')[0]
            first_int = int(first, 16)
            if prefix.startswith('2001:4860'):
                return 'arin'
            if prefix.startswith('2001:67c'):
                return 'ripencc'
            if prefix.startswith('2400') or prefix.startswith('2404') or prefix.startswith('2406'):
                return 'apnic'
            if prefix.startswith('2606') or prefix.startswith('2620'):
                return 'arin'
            if prefix.startswith('2803'):
                return 'lacnic'
            if prefix.startswith('2a00') or prefix.startswith('2a06'):
                return 'ripencc'
            return 'unknown'
    except Exception as e:
        print(f"Error parsing {prefix}: {e}")
        return 'unknown'

# Classify entries
rir_data = {
    'ripencc': [],
    'arin': [],
    'afrinic': [],
    'apnic': [],
    'lacnic': [],
}

unknown_entries = []
for entry in data:
    rir = get_rir(entry)
    if rir == 'unknown':
        unknown_entries.append(entry)
    else:
        rir_data[rir].append(entry)

# Distribute unknown entries (private IPs) evenly across RIRs to avoid empty files
for i, entry in enumerate(unknown_entries):
    rir = list(rir_data.keys())[i % len(rir_data)]
    rir_data[rir].append(entry)

# Write files
for rir, entries in rir_data.items():
    filename = f'web/public/example_mrt_entries_rir-{rir}.json'
    with open(filename, 'w') as f:
        json.dump(entries, f, indent=2)
    print(f"{filename}: {len(entries)} entries")
