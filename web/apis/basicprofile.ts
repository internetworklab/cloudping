import { useQuery } from "@tanstack/react-query";

export interface BasicProfile {
  subject_id: string;
  username: string;
}

export function useBasicProfile() {
  return useQuery<BasicProfile | null>({
    queryKey: ["basic-profile"],
    queryFn: async () => {
      const res = await fetch("/login/profile");
      if (res.status === 401) {
        throw new Error("Unauthenticated, retry after login");
      }
      if (!res.ok) {
        throw new Error(`Failed to fetch profile: ${res.status}`);
      }
      return (await res.json()) as BasicProfile;
    },
    retry: false,
    refetchOnWindowFocus: false,
  });
}
