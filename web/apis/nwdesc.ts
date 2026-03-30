export interface NetworkDescriptor {
  // uniquely identify each routes, unique across the list (or better, globally unique)
  id: string;

  // could be 192.168.0.0/16, 2001:db8:1234::/48
  prefix: string;

  // name doesn't have to be unique, however better to make it descriptive
  name: string;

  description?: string;
}

export interface DomainDescriptor {
  // uniquely identify each domain descriptor
  id: string;

  // could be, for example: dn42, dn42., com, com., example.com, www.example.com.
  zone: string;

  // name doesn't have to be unique, however better to make it descriptive
  name: string;

  description?: string;
}

function normalize(zoneOrDomain: string): string {
  const lower = zoneOrDomain.toLowerCase();
  return lower.endsWith(".") ? lower : lower + ".";
}

/**
 * DNS zone matching: finds all DomainDescriptors whose zone is a suffix of
 * the given domain. Results are sorted by specificity (longest zone first).
 *
 * Matching is case-insensitive and normalizes trailing dots.
 * For example, querying "www.example.com." matches zones
 * "example.com.", "com.", etc.
 */
export function domainDescriptorLookup(
  descriptors: DomainDescriptor[],
  domain: string,
): DomainDescriptor[] {
  const fqdn = normalize(domain);

  const matches = descriptors.filter((desc) => {
    const zoneFqdn = normalize(desc.zone);
    return fqdn === zoneFqdn || fqdn.endsWith("." + zoneFqdn);
  });

  // Sort by longest zone first (most specific match)
  matches.sort((a, b) => b.zone.length - a.zone.length);
  return matches;
}
