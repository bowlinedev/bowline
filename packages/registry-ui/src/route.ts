export type Route =
  | { view: "services" }
  | { view: "service"; name: string }
  | { view: "contract"; name: string; hash: string }
  | { view: "graph" }
  | { view: "impact"; name?: string };

export function parse(hash: string): Route {
  const segments = hash
    .replace(/^#\/?/, "")
    .split("/")
    .filter((s) => s !== "")
    .map(decodeURIComponent);
  if (segments[0] === "graph") {
    return { view: "graph" };
  }
  if (segments[0] === "impact") {
    return segments[1] === undefined ? { view: "impact" } : { view: "impact", name: segments[1] };
  }
  if (segments[0] === "services" && segments[1] !== undefined) {
    if (segments[2] === "versions" && segments[3] !== undefined) {
      return { view: "contract", name: segments[1], hash: segments[3] };
    }
    return { view: "service", name: segments[1] };
  }
  return { view: "services" };
}

export function href(route: Route): string {
  const e = encodeURIComponent;
  switch (route.view) {
    case "services":
      return "#/";
    case "graph":
      return "#/graph";
    case "impact":
      return route.name === undefined ? "#/impact" : `#/impact/${e(route.name)}`;
    case "service":
      return `#/services/${e(route.name)}`;
    case "contract":
      return `#/services/${e(route.name)}/versions/${e(route.hash)}`;
  }
}
