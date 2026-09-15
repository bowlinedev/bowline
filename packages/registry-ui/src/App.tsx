import { useEffect, useState } from "react";
import { href, parse, type Route } from "./route.js";
import { ContractView, GraphView, ImpactView, ServiceDetailView, ServiceList } from "./views.js";

function current(): Route {
  return parse(window.location.hash);
}

function View({ route }: { route: Route }) {
  switch (route.view) {
    case "services":
      return <ServiceList />;
    case "service":
      return <ServiceDetailView name={route.name} />;
    case "contract":
      return <ContractView name={route.name} hash={route.hash} />;
    case "graph":
      return <GraphView />;
    case "impact":
      return <ImpactView name={route.name} />;
  }
}

export function App() {
  const [route, setRoute] = useState<Route>(current);
  useEffect(() => {
    const onHashChange = () => setRoute(current());
    window.addEventListener("hashchange", onHashChange);
    return () => window.removeEventListener("hashchange", onHashChange);
  }, []);
  return (
    <div className="layout">
      <header>
        <h1>Bowline registry</h1>
        <nav>
          <a href={href({ view: "services" })} aria-current={route.view === "services"}>
            Services
          </a>
          <a href={href({ view: "graph" })} aria-current={route.view === "graph"}>
            Dependencies
          </a>
          <a href={href({ view: "impact" })} aria-current={route.view === "impact"}>
            Impact
          </a>
        </nav>
      </header>
      <main>
        <View route={route} />
      </main>
    </div>
  );
}
