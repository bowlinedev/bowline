import { mkdir, writeFile } from "node:fs/promises";
import { dirname } from "node:path";
import { canonicalInput, type Interaction, type RecordSink } from "./record.js";

export interface FileSinkOptions {
  provider?: string;
  version?: string;
}

export interface FileSink extends RecordSink {
  flush(): Promise<void>;
  interactions(): Interaction[];
}

export function fileSink(consumer: string, path: string, options: FileSinkOptions = {}): FileSink {
  const seen = new Map<string, Interaction>();
  return {
    consumer,
    write(interaction: Interaction) {
      const key = `${interaction.procedure}\n${canonicalInput(interaction.input)}`;
      if (!seen.has(key)) {
        seen.set(key, interaction);
      }
    },
    interactions() {
      return [...seen.values()];
    },
    async flush() {
      const document: Record<string, unknown> = {
        bowline: options.version ?? "1.4",
        consumer,
      };
      if (options.provider !== undefined) {
        document.provider = options.provider;
      }
      document.interactions = [...seen.values()].sort((a, b) =>
        a.procedure === b.procedure
          ? canonicalInput(a.input).localeCompare(canonicalInput(b.input))
          : a.procedure.localeCompare(b.procedure),
      );
      await mkdir(dirname(path), { recursive: true });
      await writeFile(path, `${JSON.stringify(document, null, 2)}\n`);
    },
  };
}
