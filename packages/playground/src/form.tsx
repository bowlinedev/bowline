import type { ContractField, ContractType, ContractTypeDecl } from "@bowlinedev/client";
import { useState } from "react";
import { type ContractIndex, typeLabel } from "./contract.js";

type Env = Record<string, ContractType>;

const MAX_DEPTH = 3;

interface Resolved {
  kind: "primitive" | "enum" | "struct" | "array" | "map" | "raw";
  primitive?: string;
  encoding?: string;
  decl?: ContractTypeDecl;
  fields?: ContractField[];
  env?: Env;
  elem?: ContractType;
  key?: ContractType;
  value?: ContractType;
  id?: string;
}

function substitute(type: ContractType, env: Env): ContractType {
  if (type.kind === "param" && type.name !== undefined && env[type.name] !== undefined) {
    return env[type.name] as ContractType;
  }
  return type;
}

export function resolve(index: ContractIndex, type: ContractType, env: Env = {}): Resolved {
  const t = substitute(type, env);
  switch (t.kind) {
    case "primitive": {
      const out: Resolved = { kind: "primitive", primitive: t.name ?? "string" };
      if (t.encoding !== undefined) {
        out.encoding = t.encoding;
      }
      if (t.name === "raw") {
        out.kind = "raw";
      }
      return out;
    }
    case "ref": {
      const decl = t.id === undefined ? undefined : index.type(t.id);
      if (decl === undefined) {
        return { kind: "raw" };
      }
      switch (decl.kind) {
        case "primitive": {
          const out: Resolved = { kind: "primitive", primitive: decl.primitive ?? "string" };
          if (t.encoding !== undefined) {
            out.encoding = t.encoding;
          }
          return out;
        }
        case "enum":
          return { kind: "enum", decl };
        case "struct":
          return { kind: "struct", decl, fields: decl.fields ?? [], env: {}, id: t.id ?? "" };
        case "generic": {
          const inner: Env = {};
          (decl.params ?? []).forEach((p, i) => {
            const arg = t.args?.[i];
            if (arg !== undefined) {
              inner[p] = substitute(arg, env);
            }
          });
          return {
            kind: "struct",
            decl,
            fields: decl.body?.fields ?? [],
            env: inner,
            id: t.id ?? "",
          };
        }
        default:
          return { kind: "raw" };
      }
    }
    case "array":
      return { kind: "array", elem: t.elem ?? { kind: "primitive", name: "string" } };
    case "map":
      return {
        kind: "map",
        key: t.key ?? { kind: "primitive", name: "string" },
        value: t.value ?? { kind: "primitive", name: "string" },
      };
    case "struct":
      return { kind: "struct", fields: t.fields ?? [], env };
    default:
      return { kind: "raw" };
  }
}

function rule(field: ContractField | undefined, name: string): string | undefined {
  const found = field?.rules?.find((r) => r.rule === name);
  return found === undefined ? undefined : (found.param ?? "");
}

export function enumOptions(
  index: ContractIndex,
  type: ContractType,
  field?: ContractField,
  env: Env = {},
): (string | number)[] | undefined {
  const oneof = rule(field, "oneof");
  const resolved = resolve(index, type, env);
  if (oneof !== undefined) {
    const options = oneof.split(/\s+/).filter((s) => s !== "");
    if (resolved.kind === "primitive" && resolved.primitive !== "string") {
      return options.map(Number);
    }
    if (
      resolved.kind === "enum" &&
      resolved.decl?.base !== undefined &&
      resolved.decl.base !== "string"
    ) {
      return options.map(Number);
    }
    return options;
  }
  if (resolved.kind === "enum") {
    return (resolved.decl?.values ?? []).map((v) => v.value);
  }
  return undefined;
}

export function initialValue(
  index: ContractIndex,
  type: ContractType,
  field?: ContractField,
  env: Env = {},
  depth: Record<string, number> = {},
): unknown {
  if (field?.example !== undefined) {
    return field.example;
  }
  const options = enumOptions(index, type, field, env);
  if (options !== undefined && options.length > 0) {
    return options[0];
  }
  const resolved = resolve(index, type, env);
  switch (resolved.kind) {
    case "primitive":
      return primitiveInitial(resolved.primitive ?? "string", resolved.encoding, field);
    case "enum":
      return "";
    case "raw":
      return {};
    case "array": {
      const min = Number(
        rule(field, "min") ??
          rule(field, "len") ??
          (rule(field, "required") !== undefined ? "1" : "0"),
      );
      const out: unknown[] = [];
      for (let i = 0; i < min; i++) {
        out.push(initialValue(index, resolved.elem as ContractType, undefined, env, depth));
      }
      return out;
    }
    case "map":
      return {};
    case "struct": {
      const id = resolved.id;
      const seen = id === undefined ? 0 : (depth[id] ?? 0);
      if (id !== undefined && seen >= MAX_DEPTH) {
        return null;
      }
      const next = id === undefined ? depth : { ...depth, [id]: seen + 1 };
      const out: Record<string, unknown> = {};
      for (const f of resolved.fields ?? []) {
        if (f.optional && f.example === undefined) {
          continue;
        }
        out[f.name] =
          f.nullable && f.example === undefined
            ? null
            : initialValue(index, f.type, f, resolved.env ?? {}, next);
      }
      return out;
    }
    default:
      return null;
  }
}

function primitiveInitial(
  name: string,
  encoding: string | undefined,
  field?: ContractField,
): unknown {
  switch (name) {
    case "string":
      return "";
    case "bool":
      return false;
    case "timestamp":
      return new Date(0).toISOString();
    case "bytes":
      return "";
    case "float32":
    case "float64": {
      const min = rule(field, "min");
      return min === undefined ? 0 : Number(min);
    }
    default: {
      const min = rule(field, "min") ?? rule(field, "len");
      const value = min === undefined ? 0 : Number(min);
      return encoding === "string" ? String(value) : value;
    }
  }
}

interface FormProps {
  index: ContractIndex;
  type: ContractType;
  value: unknown;
  onChange(value: unknown): void;
}

export function Form({ index, type, value, onChange }: FormProps) {
  return (
    <Node
      index={index}
      type={type}
      field={undefined}
      env={{}}
      value={value}
      onChange={onChange}
      label=""
      depth={{}}
    />
  );
}

interface NodeProps {
  index: ContractIndex;
  type: ContractType;
  field: ContractField | undefined;
  env: Env;
  value: unknown;
  onChange(value: unknown): void;
  label: string;
  depth: Record<string, number>;
}

function Node({ index, type, field, env, value, onChange, label, depth }: NodeProps) {
  const options = enumOptions(index, type, field, env);
  if (options !== undefined) {
    return (
      <select
        aria-label={label}
        value={String(value ?? "")}
        onChange={(e) =>
          onChange(typeof options[0] === "number" ? Number(e.target.value) : e.target.value)
        }
      >
        {options.map((o) => (
          <option key={String(o)} value={String(o)}>
            {String(o)}
          </option>
        ))}
      </select>
    );
  }
  const resolved = resolve(index, type, env);
  switch (resolved.kind) {
    case "primitive":
      return (
        <Primitive
          name={resolved.primitive ?? "string"}
          encoding={resolved.encoding}
          value={value}
          onChange={onChange}
          label={label}
        />
      );
    case "enum":
      return (
        <input
          aria-label={label}
          value={String(value ?? "")}
          onChange={(e) => onChange(e.target.value)}
        />
      );
    case "raw":
      return <JsonInput value={value} onChange={onChange} label={label} />;
    case "array":
      return (
        <ArrayNode
          index={index}
          elem={resolved.elem as ContractType}
          env={env}
          value={value}
          onChange={onChange}
          label={label}
          depth={depth}
        />
      );
    case "map":
      return (
        <MapNode
          index={index}
          valueType={resolved.value as ContractType}
          env={env}
          value={value}
          onChange={onChange}
          label={label}
          depth={depth}
        />
      );
    case "struct": {
      const id = resolved.id;
      const seen = id === undefined ? 0 : (depth[id] ?? 0);
      if (id !== undefined && seen >= MAX_DEPTH) {
        return <span className="muted">depth limit</span>;
      }
      const next = id === undefined ? depth : { ...depth, [id]: seen + 1 };
      return (
        <StructNode
          index={index}
          fields={resolved.fields ?? []}
          env={resolved.env ?? {}}
          value={value}
          onChange={onChange}
          depth={next}
        />
      );
    }
    default:
      return null;
  }
}

interface PrimitiveProps {
  name: string;
  encoding: string | undefined;
  value: unknown;
  onChange(value: unknown): void;
  label: string;
}

function Primitive({ name, encoding, value, onChange, label }: PrimitiveProps) {
  if (name === "bool") {
    return (
      <input
        aria-label={label}
        type="checkbox"
        checked={value === true}
        onChange={(e) => onChange(e.target.checked)}
      />
    );
  }
  const numeric =
    name !== "string" && name !== "timestamp" && name !== "bytes" && encoding !== "string";
  if (numeric) {
    return (
      <input
        aria-label={label}
        type="number"
        step={name.startsWith("float") ? "any" : 1}
        value={typeof value === "number" ? value : ""}
        onChange={(e) => onChange(e.target.value === "" ? 0 : Number(e.target.value))}
      />
    );
  }
  return (
    <input
      aria-label={label}
      value={typeof value === "string" ? value : ""}
      placeholder={name}
      onChange={(e) => onChange(e.target.value)}
    />
  );
}

interface StructProps {
  index: ContractIndex;
  fields: ContractField[];
  env: Env;
  value: unknown;
  onChange(value: unknown): void;
  depth: Record<string, number>;
}

function StructNode({ index, fields, env, value, onChange, depth }: StructProps) {
  const obj =
    typeof value === "object" && value !== null && !Array.isArray(value)
      ? (value as Record<string, unknown>)
      : {};
  const set = (name: string, v: unknown) => onChange({ ...obj, [name]: v });
  const unset = (name: string) => {
    const copy = { ...obj };
    delete copy[name];
    onChange(copy);
  };
  return (
    <div className="struct">
      {fields.map((f) => {
        const present = Object.hasOwn(obj, f.name);
        const isNull = present && obj[f.name] === null;
        return (
          <div className="field" key={f.name}>
            <div className="field-head">
              {f.optional && (
                <input
                  type="checkbox"
                  aria-label={`include ${f.name}`}
                  checked={present}
                  onChange={(e) =>
                    e.target.checked
                      ? set(f.name, initialValue(index, f.type, f, env, depth))
                      : unset(f.name)
                  }
                />
              )}
              <span className="field-name">{f.name}</span>
              <span className="muted">{typeLabel(index, substitute(f.type, env))}</span>
              {f.nullable && present && (
                <label className="muted">
                  <input
                    type="checkbox"
                    checked={isNull}
                    onChange={(e) =>
                      set(
                        f.name,
                        e.target.checked ? null : initialValue(index, f.type, f, env, depth),
                      )
                    }
                  />{" "}
                  null
                </label>
              )}
              {f.rules !== undefined && f.rules.length > 0 && (
                <span className="rules">
                  {f.rules
                    .map((r) =>
                      r.param === undefined || r.param === "" ? r.rule : `${r.rule}=${r.param}`,
                    )
                    .join(" ")}
                </span>
              )}
            </div>
            {f.doc !== undefined && f.doc !== "" && <div className="doc">{f.doc}</div>}
            {present && !isNull && (
              <Collapsible type={substitute(f.type, env)} index={index}>
                <Node
                  index={index}
                  type={f.type}
                  field={f}
                  env={env}
                  value={obj[f.name]}
                  onChange={(v) => set(f.name, v)}
                  label={f.name}
                  depth={depth}
                />
              </Collapsible>
            )}
          </div>
        );
      })}
      {fields.length === 0 && <span className="muted">no fields</span>}
    </div>
  );
}

function Collapsible({
  type,
  index,
  children,
}: {
  type: ContractType;
  index: ContractIndex;
  children: React.ReactNode;
}) {
  const resolved = resolve(index, type);
  const [open, setOpen] = useState(true);
  if (resolved.kind !== "struct") {
    return <>{children}</>;
  }
  return (
    <div className="nested">
      <button type="button" className="link" onClick={() => setOpen(!open)}>
        {open ? "collapse" : "expand"}
      </button>
      {open && children}
    </div>
  );
}

interface ArrayProps {
  index: ContractIndex;
  elem: ContractType;
  env: Env;
  value: unknown;
  onChange(value: unknown): void;
  label: string;
  depth: Record<string, number>;
}

function ArrayNode({ index, elem, env, value, onChange, label, depth }: ArrayProps) {
  const list = Array.isArray(value) ? value : [];
  return (
    <div className="list">
      {list.map((item, i) => (
        <div className="list-item" key={String(i)}>
          <Node
            index={index}
            type={elem}
            field={undefined}
            env={env}
            value={item}
            onChange={(v) => onChange(list.map((x, j) => (j === i ? v : x)))}
            label={`${label}[${i}]`}
            depth={depth}
          />
          <button
            type="button"
            className="link"
            onClick={() => onChange(list.filter((_, j) => j !== i))}
          >
            remove
          </button>
        </div>
      ))}
      <button
        type="button"
        className="link"
        onClick={() => onChange([...list, initialValue(index, elem, undefined, env, depth)])}
      >
        add
      </button>
    </div>
  );
}

interface MapProps {
  index: ContractIndex;
  valueType: ContractType;
  env: Env;
  value: unknown;
  onChange(value: unknown): void;
  label: string;
  depth: Record<string, number>;
}

function MapNode({ index, valueType, env, value, onChange, label, depth }: MapProps) {
  const obj =
    typeof value === "object" && value !== null && !Array.isArray(value)
      ? (value as Record<string, unknown>)
      : {};
  const entries = Object.entries(obj);
  const rename = (from: string, to: string) => {
    const next: Record<string, unknown> = {};
    for (const [k, v] of entries) {
      next[k === from ? to : k] = v;
    }
    onChange(next);
  };
  return (
    <div className="list">
      {entries.map(([k, v]) => (
        <div className="list-item" key={k}>
          <input
            aria-label={`${label} key`}
            value={k}
            onChange={(e) => rename(k, e.target.value)}
          />
          <Node
            index={index}
            type={valueType}
            field={undefined}
            env={env}
            value={v}
            onChange={(nv) => onChange({ ...obj, [k]: nv })}
            label={`${label}[${k}]`}
            depth={depth}
          />
          <button
            type="button"
            className="link"
            onClick={() => {
              const copy = { ...obj };
              delete copy[k];
              onChange(copy);
            }}
          >
            remove
          </button>
        </div>
      ))}
      <button
        type="button"
        className="link"
        onClick={() =>
          onChange({
            ...obj,
            [`key${entries.length + 1}`]: initialValue(index, valueType, undefined, env, depth),
          })
        }
      >
        add
      </button>
    </div>
  );
}

function JsonInput({
  value,
  onChange,
  label,
}: {
  value: unknown;
  onChange(value: unknown): void;
  label: string;
}) {
  const [text, setText] = useState(JSON.stringify(value ?? {}));
  const [bad, setBad] = useState(false);
  return (
    <textarea
      aria-label={label}
      className={bad ? "bad" : ""}
      value={text}
      onChange={(e) => {
        setText(e.target.value);
        try {
          onChange(JSON.parse(e.target.value));
          setBad(false);
        } catch {
          setBad(true);
        }
      }}
    />
  );
}
