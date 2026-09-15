import { useEffect, useId, useMemo, useRef, useState } from "react";
import {
  type Consumer,
  type Graph,
  getContract,
  getGraph,
  getService,
  type ImpactReport,
  listConsumers,
  listServices,
  listVersions,
  queryImpact,
  type Service,
  type ServiceDetail,
  type VersionSummary,
} from "./api.js";
import { type ContractIndex, indexContract } from "./contract.js";
import { layout, nodeHeight, nodeWidth } from "./graph.js";
import { byConsumer, changeText, verdict } from "./impact.js";
import { href } from "./route.js";
import { TypesPane } from "./types-pane.js";

interface Loaded<T> {
  key: string;
  value?: T;
  error?: string;
}

function useLoad<T>(key: string, load: () => Promise<T>): Loaded<T> {
  const [state, setState] = useState<Loaded<T>>({ key });
  const latest = useRef(load);
  latest.current = load;
  useEffect(() => {
    let live = true;
    setState({ key });
    latest.current().then(
      (value) => {
        if (live) {
          setState({ key, value });
        }
      },
      (err: unknown) => {
        if (live) {
          setState({ key, error: String(err) });
        }
      },
    );
    return () => {
      live = false;
    };
  }, [key]);
  return state;
}

function Status<T>({ state }: { state: Loaded<T> }) {
  if (state.error !== undefined) {
    return <p className="error">{state.error}</p>;
  }
  if (state.value === undefined) {
    return <p className="muted">Loading…</p>;
  }
  return null;
}

function when(iso: string): string {
  const date = new Date(iso);
  return Number.isNaN(date.valueOf()) ? iso : date.toISOString().replace("T", " ").slice(0, 19);
}

export function ServiceList() {
  const state = useLoad<Service[]>("services", listServices);
  const services = state.value;
  return (
    <section>
      <h2>Services</h2>
      <Status state={state} />
      {services !== undefined && services.length === 0 ? (
        <p className="muted">No services have been published yet.</p>
      ) : null}
      <ul className="cards" data-testid="services">
        {(services ?? []).map((service) => (
          <li key={service.name}>
            <a href={href({ view: "service", name: service.name })}>{service.name}</a>
            {service.description === undefined ? null : <p>{service.description}</p>}
            <dl>
              <dt>Owners</dt>
              <dd>{service.owners?.join(", ") ?? "—"}</dd>
              <dt>Created</dt>
              <dd>{when(service.createdAt)}</dd>
            </dl>
            <Latest name={service.name} />
          </li>
        ))}
      </ul>
    </section>
  );
}

function Latest({ name }: { name: string }) {
  const state = useLoad<ServiceDetail>(`service:${name}`, () => getService(name));
  const latest = state.value?.latest;
  if (latest === undefined) {
    return <p className="muted">no versions</p>;
  }
  return (
    <p className="mono latest">
      <a href={href({ view: "contract", name, hash: latest.hash })}>{latest.hash}</a>
    </p>
  );
}

export function ServiceDetailView({ name }: { name: string }) {
  const versions = useLoad<VersionSummary[]>(`versions:${name}`, () => listVersions(name));
  const consumers = useLoad<Consumer[]>(`consumers:${name}`, () => listConsumers(name));
  return (
    <section>
      <h2>{name}</h2>
      <p>
        <a href={href({ view: "impact", name })}>Run an impact query against this service</a>
      </p>
      <h3>Versions</h3>
      <Status state={versions} />
      <table data-testid="versions">
        <thead>
          <tr>
            <th>Hash</th>
            <th>Published</th>
            <th>Ref</th>
            <th>Tags</th>
          </tr>
        </thead>
        <tbody>
          {(versions.value ?? []).map((version) => (
            <tr key={version.hash}>
              <td className="mono">
                <a href={href({ view: "contract", name, hash: version.hash })}>{version.hash}</a>
              </td>
              <td>{when(version.publishedAt)}</td>
              <td className="mono">{version.ref ?? "—"}</td>
              <td>{version.tags?.join(", ") ?? "—"}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <h3>Consumers</h3>
      <Status state={consumers} />
      {consumers.value !== undefined && consumers.value.length === 0 ? (
        <p className="muted">No consumer has recorded usage of {name}.</p>
      ) : null}
      <table data-testid="consumers">
        <tbody>
          {(consumers.value ?? []).map((consumer) => (
            <tr key={`${consumer.consumer}-${consumer.recordedAt}`}>
              <td>{consumer.consumer}</td>
              <td className="mono">{consumer.providerHash ?? "—"}</td>
              <td>{when(consumer.recordedAt)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}

export function ContractView({ name, hash }: { name: string; hash: string }) {
  const state = useLoad<ContractIndex>(`contract:${name}:${hash}`, async () =>
    indexContract(await getContract(name, hash)),
  );
  const index = state.value;
  return (
    <section>
      <h2>
        <a href={href({ view: "service", name })}>{name}</a>
      </h2>
      <p className="mono">{hash}</p>
      <Status state={state} />
      {index === undefined ? null : (
        <div className="contract">
          <div>
            <h3>Procedures</h3>
            <table data-testid="procedures">
              <tbody>
                {index.procedures.map((procedure) => (
                  <tr key={procedure.path}>
                    <td className="mono">{procedure.path}</td>
                    <td>{procedure.kind}</td>
                    <td>{procedure.method}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div>
            <h3>Types</h3>
            <TypesPane index={index} />
          </div>
        </div>
      )}
    </section>
  );
}

const edgeColors: Record<string, string> = { consumes: "#2563eb", composes: "#a855f7" };

export function GraphView() {
  const state = useLoad<Graph>("graph", getGraph);
  const graph = state.value;
  const placed = useMemo(() => (graph === undefined ? undefined : layout(graph)), [graph]);
  return (
    <section>
      <h2>Dependencies</h2>
      <Status state={state} />
      {placed === undefined ? null : (
        <svg
          className="graph"
          role="img"
          aria-label="dependency graph"
          viewBox={`0 0 ${placed.width} ${placed.height}`}
          width={placed.width}
          height={placed.height}
        >
          <title>Dependency graph</title>
          {placed.edges.map((edge) => (
            <line
              key={`${edge.kind}-${edge.from}-${edge.to}`}
              data-testid={`edge-${edge.from}-${edge.to}`}
              x1={edge.x1}
              y1={edge.y1}
              x2={edge.x2}
              y2={edge.y2}
              stroke={edgeColors[edge.kind] ?? "#888"}
              strokeWidth={1.5}
            />
          ))}
          {placed.nodes.map((node) => (
            <g key={node.name} data-testid={`node-${node.name}`}>
              <rect
                x={node.x}
                y={node.y}
                width={nodeWidth}
                height={nodeHeight}
                rx={6}
                fill="none"
                stroke="#888"
              />
              <text x={node.x + 12} y={node.y + 18} fontSize={12}>
                {node.name}
              </text>
              <text x={node.x + 12} y={node.y + 32} fontSize={10} fill="#777">
                {node.kind}
              </text>
            </g>
          ))}
        </svg>
      )}
    </section>
  );
}

export function ImpactView({ name }: { name?: string | undefined }) {
  const services = useLoad<Service[]>("services", listServices);
  const [service, setService] = useState(name ?? "");
  const [candidate, setCandidate] = useState("");
  const [report, setReport] = useState<ImpactReport>();
  const [error, setError] = useState<string>();
  const [pending, setPending] = useState(false);
  const known = services.value;
  const ids = useId();

  useEffect(() => {
    if (name !== undefined) {
      setService(name);
    }
  }, [name]);
  useEffect(() => {
    const first = known?.[0]?.name;
    setService((current) => (current === "" && first !== undefined ? first : current));
  }, [known]);

  const submit = async () => {
    setPending(true);
    setError(undefined);
    setReport(undefined);
    try {
      setReport(await queryImpact(service, candidate));
    } catch (err: unknown) {
      setError(String(err));
    } finally {
      setPending(false);
    }
  };

  return (
    <section>
      <h2>Impact</h2>
      <p className="muted">
        Paste or upload a candidate contract document. The registry diffs it against the version
        tagged <code>main</code> and names every consumer whose recorded usage the change would
        break.
      </p>
      <label htmlFor={`${ids}-service`}>Service</label>
      <select id={`${ids}-service`} value={service} onChange={(e) => setService(e.target.value)}>
        {(known ?? []).map((s) => (
          <option key={s.name} value={s.name}>
            {s.name}
          </option>
        ))}
      </select>
      <label htmlFor={`${ids}-candidate`}>Contract document</label>
      <textarea
        id={`${ids}-candidate`}
        rows={10}
        spellCheck={false}
        value={candidate}
        onChange={(e) => setCandidate(e.target.value)}
      />
      <div className="row">
        <input
          type="file"
          accept="application/json,.json"
          aria-label="Upload a JSON file"
          onChange={async (e) => {
            const file = e.target.files?.[0];
            if (file !== undefined) {
              setCandidate(await file.text());
            }
          }}
        />
        <button
          type="button"
          disabled={pending || service === "" || candidate === ""}
          onClick={submit}
        >
          {pending ? "Checking…" : "Check impact"}
        </button>
      </div>
      {error === undefined ? null : <p className="error">{error}</p>}
      {report === undefined ? null : (
        <div data-testid="report">
          <p className={report.ok ? "ok" : "error"}>{verdict(report)}</p>
          {report.baseline === "" ? null : <p className="mono">baseline {report.baseline}</p>}
          {byConsumer(report).map((group) => (
            <div key={`${group.consumer}-${group.via ?? ""}`} className="affected">
              <h3>
                {group.consumer}
                {group.via === undefined ? "" : ` through ${group.via}`}
              </h3>
              <ul>
                {group.hits.map((hit) => (
                  <li key={`${hit.change.path}-${hit.reason}`}>
                    <span className="mono">{changeText(hit.change)}</span> — {hit.reason}
                  </li>
                ))}
              </ul>
            </div>
          ))}
          {report.unattributed.length === 0 ? null : (
            <div className="unattributed">
              <h3>Unattributed</h3>
              <ul>
                {report.unattributed.map((change) => (
                  <li key={change.path} className="mono">
                    {changeText(change)}
                  </li>
                ))}
              </ul>
            </div>
          )}
          {report.skippedCompositions === true ? (
            <p className="muted">
              Compositions were not recomposed; this registry has no composer configured.
            </p>
          ) : null}
        </div>
      )}
    </section>
  );
}
