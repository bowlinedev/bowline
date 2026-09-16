import type { ContractDocument } from "@bowlinedev/client";

export interface Service {
  name: string;
  description?: string;
  owners?: string[];
  createdAt: string;
}

export interface VersionSummary {
  hash: string;
  publishedAt: string;
  ref?: string;
  tags?: string[];
}

export interface ServiceDetail extends Service {
  latest?: VersionSummary;
}

export interface Consumer {
  consumer: string;
  provider: string;
  providerHash?: string;
  recordedAt: string;
}

export type ChangeCategory = "breaking" | "narrowed" | "widened" | "compatible" | string;

export interface Change {
  category: ChangeCategory;
  path: string;
  message: string;
}

export interface Affected {
  consumer: string;
  via?: string;
  change: Change;
  reason: string;
}

export interface ImpactReport {
  baseline: string;
  changes: Change[];
  affected: Affected[];
  unattributed: Change[];
  ok: boolean;
  skippedCompositions?: boolean;
}

export type NodeKind = "service" | "gateway" | "consumer";

export interface GraphNode {
  name: string;
  kind: NodeKind;
}

export interface GraphEdge {
  from: string;
  to: string;
  kind: "consumes" | "composes" | string;
}

export interface Graph {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

const base = new URL("./", document.baseURI);

function url(path: string): string {
  return new URL(path.replace(/^\//, ""), base).toString();
}

async function request(path: string, init?: RequestInit): Promise<Response> {
  const response = await fetch(url(path), init);
  if (!response.ok) {
    const text = await response.text();
    throw new Error(`${path} answered ${response.status}: ${text.slice(0, 400)}`);
  }
  return response;
}

async function json<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await request(path, init);
  return (await response.json()) as T;
}

export function listServices(): Promise<Service[]> {
  return json<Service[]>("v1/services");
}

export function getService(name: string): Promise<ServiceDetail> {
  return json<ServiceDetail>(`v1/services/${encodeURIComponent(name)}`);
}

export function listVersions(name: string): Promise<VersionSummary[]> {
  return json<VersionSummary[]>(`v1/services/${encodeURIComponent(name)}/versions`);
}

export function listConsumers(name: string): Promise<Consumer[]> {
  return json<Consumer[]>(`v1/services/${encodeURIComponent(name)}/consumers`);
}

export function getContract(name: string, hash: string): Promise<ContractDocument> {
  return json<ContractDocument>(
    `v1/services/${encodeURIComponent(name)}/versions/${encodeURIComponent(hash)}`,
  );
}

export function getGraph(): Promise<Graph> {
  return json<Graph>("v1/graph");
}

export function queryImpact(name: string, document: string): Promise<ImpactReport> {
  return json<ImpactReport>(`v1/services/${encodeURIComponent(name)}/impact`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: document,
  });
}
