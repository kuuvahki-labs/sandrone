import { expectTypeOf, it } from "vitest";

import type { ConvertRequest, RemoteBatchInput, RemoteInput } from "./client";

it("accepts each conversion input and requires a target format", () => {
  expectTypeOf<{ content: string; from_format: string; to_format: string }>().toExtend<ConvertRequest>();
  expectTypeOf<{ remote: RemoteInput; to_format: string }>().toExtend<ConvertRequest>();
  expectTypeOf<{ remote: RemoteInput; from_format: string; to_format: string }>().toExtend<ConvertRequest>();
  expectTypeOf<{ remote_batch: RemoteBatchInput; to_format: string }>().toExtend<ConvertRequest>();
  expectTypeOf<{ remote_batch: RemoteBatchInput; from_format: string; to_format: string }>().toExtend<ConvertRequest>();
  expectTypeOf<{ content: string; from_format: string }>().not.toExtend<ConvertRequest>();
  expectTypeOf<{ remote: RemoteInput }>().not.toExtend<ConvertRequest>();
  expectTypeOf<{ remote_batch: RemoteBatchInput }>().not.toExtend<ConvertRequest>();
});

it("rejects missing inputs, mixed inputs, body format omissions and incomplete remote sources", () => {
  type Target = { to_format: string };
  type Body = { content: string; from_format: string };
  type Single = { remote: RemoteInput };
  type Batch = { remote_batch: RemoteBatchInput };

  expectTypeOf<Target>().not.toExtend<ConvertRequest>();
  expectTypeOf<Target & { content: string }>().not.toExtend<ConvertRequest>();
  expectTypeOf<Target & Body & Single>().not.toExtend<ConvertRequest>();
  expectTypeOf<Target & Body & Batch>().not.toExtend<ConvertRequest>();
  expectTypeOf<Target & Single & Batch>().not.toExtend<ConvertRequest>();
  expectTypeOf<Target & Body & Single & Batch>().not.toExtend<ConvertRequest>();
  expectTypeOf<Target & { remote: { timeout_ms: number } }>().not.toExtend<ConvertRequest>();
  expectTypeOf<Target & { remote_batch: { sources: Array<{ url?: string }> } }>().not.toExtend<ConvertRequest>();
});
