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
