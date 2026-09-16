import type { ContractDocument, ContractProcedure } from "@bowlinedev/client";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  type ContractIndex,
  indexContract,
  shortName,
  type TreeNode,
  typeLabel,
} from "./contract.js";
import { Form, initialValue } from "./form.js";
import { type HistoryEntry, load, loadHeaders, push, save, saveHeaders } from "./history.js";
import { TypesPane } from "./types-pane.js";
import { decodeState, encodeState } from "./url-state.js";

interface Response {
  status: number;
  durationMs: number;
  body: unknown;
  text: string;
}

const TIMESTAMP = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$/;

function timestamps(
  value: unknown,
  path: string[] = [],
  out: { path: string; iso: string }[] = [],
): { path: string; iso: string }[] {
  if (typeof value === "string" && TIMESTAMP.test(value)) {
    out.push({ path: path.join("."), iso: value });
  } else if (Array.isArray(value)) {
    for (const [i, v] of value.entries()) {
      timestamps(v, [...path, String(i)], out);
    }
  } else if (typeof value === "object" && value !== null) {
    for (const [k, v] of Object.entries(value)) {
      timestamps(v, [...path, k], out);
    }
  }
  return out;
}

export function App() {
  const [index, setIndex] = useState<ContractIndex>();
  const [loadError, setLoadError] = useState<string>();
  const [selected, setSelected] = useState<string>();
  const [input, setInput] = useState<unknown>({});
  const [raw, setRaw] = useState("{}");
  const [rawBad, setRawBad] = useState(false);
  const [headers, setHeaders] = useState<Record<string, string>>(() => loadHeaders());
  const [response, setResponse] = useState<Response>();
  const [pending, setPending] = useState(false);
  const [history, setHistory] = useState<HistoryEntry[]>(() => load());
  const [pane, setPane] = useState<"call" | "types">("call");
  const [copied, setCopied] = useState(false);

  const choose = useCallback((idx: ContractIndex, proc: ContractProcedure, value?: unknown) => {
    setSelected(proc.path);
    const next = value ?? initialValue(idx, proc.input);
    setInput(next);
    setRaw(JSON.stringify(next, null, 2));
    setRawBad(false);
    setResponse(undefined);
  }, []);

  useEffect(() => {
    fetch("./contract.json", { cache: "no-store" })
      .then(async (r) => {
        if (!r.ok) {
          throw new Error(`contract.json returned ${r.status}`);
        }
        return (await r.json()) as ContractDocument;
      })
      .then((doc) => {
        const idx = indexContract(doc);
        setIndex(idx);
        const state = decodeState(window.location.hash);
        const proc = state?.procedure !== undefined ? idx.procedure(state.procedure) : undefined;
        if (proc !== undefined) {
          choose(idx, proc, state?.input);
          if (state?.headers !== undefined) {
            setHeaders((h) => ({ ...h, ...state.headers }));
          }
        } else if (idx.procedures[0] !== undefined) {
          choose(idx, idx.procedures[0]);
        }
      })
      .catch((err: unknown) => setLoadError(err instanceof Error ? err.message : String(err)));
  }, [choose]);

  const procedure = useMemo(
    () => (index !== undefined && selected !== undefined ? index.procedure(selected) : undefined),
    [index, selected],
  );

  useEffect(() => {
    if (selected === undefined) {
      return;
    }
    const encoded = encodeState({ procedure: selected, input, headers });
    window.history.replaceState(null, "", `#${encoded}`);
  }, [selected, input, headers]);

  useEffect(() => {
    saveHeaders(headers);
  }, [headers]);

  function updateInput(value: unknown) {
    setInput(value);
    setRaw(JSON.stringify(value, null, 2));
    setRawBad(false);
  }

  function updateRaw(text: string) {
    setRaw(text);
    try {
      setInput(JSON.parse(text));
      setRawBad(false);
    } catch {
      setRawBad(true);
    }
  }

  async function send() {
    if (procedure === undefined || rawBad) {
      return;
    }
    setPending(true);
    const started = performance.now();
    const init: RequestInit = {
      method: procedure.method,
      headers: { accept: "application/json", ...headers },
    };
    let url = `./proxy/${procedure.path}`;
    const body = JSON.stringify(input ?? {});
    if (procedure.method === "GET") {
      url += `?input=${encodeURIComponent(body)}`;
    } else {
      init.headers = {
        ...(init.headers as Record<string, string>),
        "content-type": "application/json",
      };
      init.body = body;
    }
    let result: Response;
    try {
      const res = await fetch(url, init);
      const text = await res.text();
      let parsed: unknown = text;
      try {
        parsed = text === "" ? null : JSON.parse(text);
      } catch {}
      result = {
        status: res.status,
        durationMs: Math.round(performance.now() - started),
        body: parsed,
        text,
      };
    } catch (err) {
      result = {
        status: 0,
        durationMs: Math.round(performance.now() - started),
        body: null,
        text: err instanceof Error ? err.message : String(err),
      };
    }
    setResponse(result);
    setPending(false);
    const entry: HistoryEntry = {
      at: new Date().toISOString(),
      procedure: procedure.path,
      input,
      status: result.status,
      durationMs: result.durationMs,
      response: result.body,
    };
    setHistory((h) => {
      const next = push(h, entry);
      save(next);
      return next;
    });
  }

  async function copyLink() {
    try {
      await navigator.clipboard.writeText(window.location.href);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {}
  }

  if (loadError !== undefined) {
    return <div className="error">Could not load the contract: {loadError}</div>;
  }
  if (index === undefined) {
    return <div className="muted">Loading contract…</div>;
  }
  const callable =
    procedure !== undefined && (procedure.kind === "query" || procedure.kind === "mutation");
  const stamps = response === undefined ? [] : timestamps(response.body);

  return (
    <div className="layout">
      <aside className="tree">
        <h1>Bowline</h1>
        <div className="muted">contract {index.doc.bowline}</div>
        <Tree nodes={index.tree} selected={selected} onSelect={(p) => choose(index, p)} />
        <nav className="panes">
          <button
            type="button"
            className={pane === "call" ? "active" : ""}
            onClick={() => setPane("call")}
          >
            Call
          </button>
          <button
            type="button"
            className={pane === "types" ? "active" : ""}
            onClick={() => setPane("types")}
          >
            Types
          </button>
        </nav>
      </aside>
      <main className="main">
        {pane === "types" && <TypesPane index={index} />}
        {pane === "call" && procedure !== undefined && (
          <div className="call">
            <header>
              <h2>
                {procedure.path} <span className={`badge ${procedure.kind}`}>{procedure.kind}</span>{" "}
                <span className="muted">{procedure.method}</span>
              </h2>
              {procedure.doc !== undefined && procedure.doc !== "" && (
                <p className="doc">{procedure.doc}</p>
              )}
              {procedure.deprecated !== undefined && procedure.deprecated !== "" && (
                <p className="warn">Deprecated: {procedure.deprecated}</p>
              )}
              {procedure.tool !== undefined && (
                <p className="muted">
                  tool{procedure.tool.readOnly ? ", read-only" : ""}
                  {procedure.tool.destructive ? ", destructive" : ""}
                  {procedure.tool.scopes !== undefined && procedure.tool.scopes.length > 0
                    ? `, scopes ${procedure.tool.scopes.join(" ")}`
                    : ""}
                </p>
              )}
              <p className="muted">
                {typeLabel(index, procedure.input)} → {typeLabel(index, procedure.output)}
                {procedure.errors !== undefined &&
                  procedure.errors.length > 0 &&
                  ` · errors ${procedure.errors.map(shortName).join(", ")}`}
              </p>
            </header>
            {!callable && (
              <p className="warn">
                The playground calls queries and mutations only; {procedure.kind}s are listed for
                reference.
              </p>
            )}
            {callable && (
              <div className="columns">
                <section>
                  <h3>Input</h3>
                  <Form index={index} type={procedure.input} value={input} onChange={updateInput} />
                  <h3>Raw JSON</h3>
                  <textarea
                    aria-label="raw input"
                    className={rawBad ? "bad" : ""}
                    value={raw}
                    onChange={(e) => updateRaw(e.target.value)}
                    rows={8}
                  />
                  <h3>Headers</h3>
                  <Headers headers={headers} onChange={setHeaders} />
                  <div className="actions">
                    <button type="button" onClick={send} disabled={pending || rawBad}>
                      Send
                    </button>
                    <button type="button" className="link" onClick={copyLink}>
                      {copied ? "copied" : "copy link"}
                    </button>
                  </div>
                </section>
                <section>
                  <h3>Response</h3>
                  {response === undefined && (
                    <div className="muted">Send a request to see the response.</div>
                  )}
                  {response !== undefined && (
                    <>
                      <div className="muted">
                        status {response.status} · {response.durationMs} ms
                      </div>
                      <pre data-testid="response">
                        {typeof response.body === "string"
                          ? response.text
                          : JSON.stringify(response.body, null, 2)}
                      </pre>
                      {stamps.length > 0 && (
                        <table className="stamps">
                          <tbody>
                            {stamps.map((s) => (
                              <tr key={s.path}>
                                <td>{s.path}</td>
                                <td>{s.iso}</td>
                                <td>{new Date(s.iso).toLocaleString()}</td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      )}
                    </>
                  )}
                  <h3>History</h3>
                  <ul className="history">
                    {history.map((h, i) => (
                      <li key={`${h.at}-${String(i)}`}>
                        <button
                          type="button"
                          className="link"
                          onClick={() => {
                            const proc = index.procedure(h.procedure);
                            if (proc !== undefined) {
                              choose(index, proc, h.input);
                            }
                          }}
                        >
                          {h.procedure}
                        </button>{" "}
                        <span className="muted">
                          {h.status} · {h.durationMs} ms · {new Date(h.at).toLocaleTimeString()}
                        </span>
                      </li>
                    ))}
                  </ul>
                </section>
              </div>
            )}
          </div>
        )}
      </main>
    </div>
  );
}

function Tree({
  nodes,
  selected,
  onSelect,
}: {
  nodes: TreeNode[];
  selected: string | undefined;
  onSelect(p: ContractProcedure): void;
}) {
  return (
    <ul>
      {nodes.map((node) => (
        <li key={node.path}>
          {node.procedure === undefined ? (
            <span className="group">{node.name}</span>
          ) : (
            <button
              type="button"
              className={`link ${selected === node.path ? "active" : ""}`}
              onClick={() => onSelect(node.procedure as ContractProcedure)}
            >
              {node.path}
            </button>
          )}
          {node.procedure !== undefined && (
            <span className={`badge ${node.procedure.kind}`}>{node.procedure.kind}</span>
          )}
          {node.children.length > 0 && (
            <Tree nodes={node.children} selected={selected} onSelect={onSelect} />
          )}
        </li>
      ))}
    </ul>
  );
}

function Headers({
  headers,
  onChange,
}: {
  headers: Record<string, string>;
  onChange(h: Record<string, string>): void;
}) {
  const entries = Object.entries(headers);
  const [name, setName] = useState("");
  const [value, setValue] = useState("");
  return (
    <div className="headers">
      {entries.map(([k, v]) => (
        <div className="list-item" key={k}>
          <code>{k}</code>
          <input
            aria-label={`header ${k}`}
            value={v}
            onChange={(e) => onChange({ ...headers, [k]: e.target.value })}
          />
          <button
            type="button"
            className="link"
            onClick={() => {
              const copy = { ...headers };
              delete copy[k];
              onChange(copy);
            }}
          >
            remove
          </button>
        </div>
      ))}
      <div className="list-item">
        <input
          aria-label="header name"
          placeholder="Authorization"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
        <input
          aria-label="header value"
          placeholder="Bearer …"
          value={value}
          onChange={(e) => setValue(e.target.value)}
        />
        <button
          type="button"
          className="link"
          onClick={() => {
            if (name.trim() !== "") {
              onChange({ ...headers, [name.trim()]: value });
              setName("");
              setValue("");
            }
          }}
        >
          add
        </button>
      </div>
    </div>
  );
}
