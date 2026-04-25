export interface RepoDetails {
  stargazers_count?: number;
  html_url?: string;
}

export function getRepo(owner: string, name: string) {
  const url = `https://api.github.com/repos/${owner}/${name}`;
  return fetch(url)
    .then((r) => r.json())
    .then((r) => r as RepoDetails);
}
