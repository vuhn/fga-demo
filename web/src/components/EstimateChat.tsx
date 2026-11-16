import {
  Accordion,
  Alert,
  Anchor,
  Badge,
  Button,
  Group,
  Loader,
  NumberInput,
  Paper,
  ScrollArea,
  Stack,
  Table,
  Text,
  Textarea,
  Tooltip,
} from "@mantine/core";
import { IconPencil, IconPlayerStop, IconPlus, IconSend } from "@tabler/icons-react";
import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import {
  api,
  ApiError,
  apiChatStream,
  type EstimateFact,
  type EstimateLine,
  type EstimateMessage,
  type EstimateQuestion,
  type EstimateRate,
  type EstimateResult,
  type EstimateSessionDetail,
  type EstimateState,
  type FactSource,
  type Progress,
  type StoredEstimate,
  type StoredEstimateSummary,
} from "../api";
import { confirmDanger, notifyError, run } from "../ui";

const money = (v: number | null | undefined) => (v == null ? "-" : new Intl.NumberFormat("vi-VN").format(Math.round(v)));
const SOURCE_COLOR: Record<FactSource, string> = { user: "blue", plan: "teal", assumed: "orange" };
const SOURCE_LABEL: Record<FactSource, string> = { user: "you", plan: "plan", assumed: "assumed" };
const FACT_LABEL: Record<string, string> = {
  kind: "Kind", work_type: "Work", city: "City", district: "District", quality: "Quality", floor_height: "Floor height (m)", storeys: "Storeys",
  structure_in_scope: "Structure", mep_in_scope: "MEP", fixtures: "Existing fixtures", excluded: "Left out", vat_percent: "VAT %", overhead_percent: "Overhead %", profit_percent: "Profit %",
  design_percent: "Design %", contingency_percent: "Contingency %",
};
const START = "Please estimate the cost of building this plan.";

/** A source link is shown only for web URLs: page text is untrusted, never render other schemes. */
const safeUrl = (u: string) => (/^https?:\/\//i.test(u) ? u : null);

type Live = { text: string; status: Progress[]; rates: EstimateRate[] };
type Room = { id: string; name: string };

/**
 * The Estimate tab: a streaming chat with the cost-estimation agent. The agent asks before it assumes,
 * researches unit prices on the web, and the engine does all the maths. Editors chat; a viewer only
 * reads finished estimates. Nothing of a turn is saved unless it completes (Stop discards it).
 */
export default function EstimateChat({ blueprintId, currentVersion, canEstimate, rooms }: { blueprintId: string; currentVersion: number; canEstimate: boolean; rooms: Room[] }) {
  const [loaded, setLoaded] = useState(false);
  const [session, setSession] = useState<EstimateSessionDetail | null>(null);
  const [messages, setMessages] = useState<EstimateMessage[]>([]);
  const [state, setState] = useState<EstimateState>({});
  const [question, setQuestion] = useState<EstimateQuestion | null>(null);
  const [live, setLive] = useState<Live | null>(null);
  const [text, setText] = useState("");
  const [saved, setSaved] = useState<StoredEstimateSummary[]>([]);
  const [viewed, setViewed] = useState<StoredEstimate | null>(null);
  const abort = useRef<AbortController | null>(null);
  const bottom = useRef<HTMLDivElement>(null);
  const busy = live !== null;

  const open = useCallback((d: EstimateSessionDetail) => {
    setSession(d);
    setMessages(d.messages);
    setState(d.state ?? {});
    setQuestion(null);
  }, []);

  const load = useCallback(async () => {
    try {
      if (!canEstimate) {
        setSaved(await api<StoredEstimateSummary[]>("GET", `/blueprints/${blueprintId}/estimates`));
        return;
      }
      open(await api<EstimateSessionDetail>("GET", `/blueprints/${blueprintId}/estimate-session`));
    } catch (e) {
      if (!(e instanceof ApiError && e.status === 404)) notifyError(e); // 404: no estimate started yet
    } finally {
      setLoaded(true);
    }
  }, [blueprintId, canEstimate, open]);

  useEffect(() => {
    void load();
    return () => abort.current?.abort();
  }, [load]);

  useEffect(() => {
    bottom.current?.scrollIntoView({ block: "end" });
  }, [messages, live?.text]);

  async function newSession() {
    let d: EstimateSessionDetail | null = null;
    await run(async () => {
      d = await api<EstimateSessionDetail>("POST", `/blueprints/${blueprintId}/estimate-session`);
    });
    if (d) open(d); // one session per blueprint: it replaced the old one
  }

  // Starting over deletes the chat and answers (finished estimates stay), so ask first when there is something to lose.
  function restart() {
    if (messages.length === 0 && !state.estimate) return void newSession();
    confirmDanger(
      "Start a new estimate?",
      "This deletes the current conversation and your answers. The last priced estimate stays visible to people who can view the blueprint. This cannot be undone.",
      "Start new",
      () => void newSession(),
    );
  }

  async function send(message: string, overrides?: Record<string, number>) {
    if (!session || busy) return;
    const body = message.trim();
    const before = messages;
    if (body) setMessages((m) => [...m, { seq: 0, role: "user", content: body, created_at: new Date().toISOString() }]);
    setQuestion(null);
    setText("");
    setLive({ text: "", status: [], rates: [] });
    const ctl = new AbortController();
    abort.current = ctl;
    try {
      const done = await apiChatStream<EstimateSessionDetail>(
        `/blueprints/${blueprintId}/estimate-session/messages`,
        { message: body, rate_overrides: overrides },
        (event, data) => {
          if (event === "token") setLive((l) => l && { ...l, text: l.text + (data as { text: string }).text });
          else if (event === "progress") setLive((l) => l && { ...l, status: [...l.status.slice(-3), data as Progress] });
          else if (event === "rate") setLive((l) => l && { ...l, rates: [...l.rates, data as EstimateRate] });
          else if (event === "question") setQuestion(data as EstimateQuestion);
          else if (event === "fact") {
            const f = data as { name: string; value: EstimateFact["value"]; source: FactSource };
            setState((st) =>
              f.name.startsWith("scope.")
                ? { ...st, scope: { ...st.scope, [f.name.slice(6)]: { state: f.value as never, source: f.source } } }
                : { ...st, facts: { ...st.facts, [f.name]: { value: f.value, source: f.source } } },
            );
          }
        },
        ctl.signal,
      );
      // the saved turn replaces everything shown live; keep the chips the agent just asked
      setSession(done);
      setMessages(done.messages);
      setState(done.state ?? {});
    } catch (e) {
      // nothing was saved: take the optimistic message back and give the text back to the person
      setMessages(before);
      setQuestion(null);
      setText(body);
      if ((e as Error).name === "AbortError") notifyError(new Error("Stopped. Nothing from that turn was saved."));
      else notifyError(e);
    } finally {
      abort.current = null;
      setLive(null);
    }
  }

  function submit(e: FormEvent) {
    e.preventDefault();
    if (text.trim()) void send(text);
  }

  // ---- read-only view for people who can view but not estimate ----
  if (!canEstimate) {
    return (
      <Stack gap="xs">
        {saved.length === 0 && <Text size="sm" c="dimmed">No estimate has been made for this blueprint yet.</Text>}
        {saved.map((e) => (
          <Paper key={e.id} withBorder p="xs" radius="md">
            <Group justify="space-between" wrap="nowrap">
              <div>
                <Text size="sm" fw={500}>{money(e.total_base)} VND</Text>
                <Text size="xs" c="dimmed">{money(e.total_low)} to {money(e.total_high)} · plan v{e.version} · {new Date(e.created_at).toLocaleString()}</Text>
              </div>
              <Button size="xs" variant="light" onClick={() => void run(async () => setViewed(await api<StoredEstimate>("GET", `/blueprints/${blueprintId}/estimates/${e.id}`)))}>
                Open
              </Button>
            </Group>
          </Paper>
        ))}
        {viewed && <ResultCard result={viewed.result} canOverride={false} onRate={() => {}} stale={viewed.version < currentVersion} version={viewed.version} current={currentVersion} />}
      </Stack>
    );
  }

  if (!loaded) return <Loader size="sm" />;
  if (!session) {
    return (
      <Stack gap="xs">
        <Text size="sm" c="dimmed">
          An AI agent estimates the build cost of this plan. It asks about what the drawing cannot show (location, what already exists, quality, scope), looks up current unit prices on the web, and the totals are computed in code from measured quantities.
        </Text>
        <Button size="xs" leftSection={<IconPlus size={14} />} onClick={() => void newSession()}>
          Start an estimate
        </Button>
      </Stack>
    );
  }

  const stale = session.blueprint_version < currentVersion;
  const result = state.estimate ?? null;
  const factEntries = Object.entries(state.facts ?? {});
  const empty = messages.length === 0 && !live;

  return (
    <Stack gap="sm">
      <Group justify="space-between">
        <Text size="xs" c="dimmed">Session on plan v{session.blueprint_version} · {state.status ?? "interviewing"}</Text>
        <Button size="compact-xs" variant="subtle" leftSection={<IconPlus size={12} />} disabled={busy} onClick={restart}>
          New estimate
        </Button>
      </Group>
      {stale && (
        <Alert color="yellow" p="xs" title="The plan changed">
          <Text size="xs">This estimate was made on v{session.blueprint_version}; the blueprint is now v{currentVersion}. Start a new estimate to price the current plan.</Text>
        </Alert>
      )}

      <ScrollArea.Autosize mah={340} type="auto">
        <Stack gap="xs" pr="xs">
          {empty && (
            <Paper withBorder p="xs" radius="md">
              <Text size="sm" c="dimmed">Say what you want, for example “This is an apartment, estimate the cost with normal quality”, or just start.</Text>
              <Button mt="xs" size="compact-xs" variant="light" onClick={() => void send(START)}>Start the estimate</Button>
            </Paper>
          )}
          {messages.map((m, i) => (
            <Bubble key={`${m.seq}-${i}`} role={m.role} text={m.content} />
          ))}
          {live && (
            <>
              {live.text && <Bubble role="assistant" text={live.text} />}
              <Paper withBorder p="xs" radius="md" bg="var(--mantine-color-gray-light)">
                <Group gap="xs">
                  <Loader size="xs" />
                  <Text size="xs" c="dimmed">{live.status.length ? live.status[live.status.length - 1].message : "Thinking…"}</Text>
                </Group>
                {live.rates.length > 0 && (
                  <Stack gap={2} mt={4}>
                    {live.rates.map((r) => (
                      <Text key={r.code} size="xs" c={r.confidence === "none" ? "red" : "dimmed"}>
                        {r.code.replace(/_/g, " ")}: {r.confidence === "none" ? "no price found" : `${money(r.base)} VND/${r.unit}${r.confidence === "low_confidence" ? " (low confidence)" : ""}`}
                      </Text>
                    ))}
                  </Stack>
                )}
              </Paper>
            </>
          )}
          <div ref={bottom} />
        </Stack>
      </ScrollArea.Autosize>

      {question && !busy && (
        <Group gap={6}>
          {question.options.map((o) => (
            <Button key={o} size="compact-xs" variant="light" radius="xl" onClick={() => void send(o)}>
              {o}
            </Button>
          ))}
        </Group>
      )}

      <form onSubmit={submit}>
        <Stack gap="xs">
          <Textarea
            placeholder={question ? "Your answer…" : "Ask or change something, e.g. “actually it’s District 7”"}
            autosize
            minRows={1}
            maxRows={4}
            maxLength={2000}
            value={text}
            onChange={(e) => setText(e.currentTarget.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey && text.trim()) {
                e.preventDefault();
                void send(text);
              }
            }}
            disabled={busy}
          />
          <Group justify="flex-end" gap="xs">
            {busy ? (
              <Button size="xs" color="red" variant="light" leftSection={<IconPlayerStop size={14} />} onClick={() => abort.current?.abort()}>
                Stop
              </Button>
            ) : (
              <Button type="submit" size="xs" leftSection={<IconSend size={14} />} disabled={!text.trim()}>
                Send
              </Button>
            )}
          </Group>
        </Stack>
      </form>

      {(factEntries.length > 0 || Object.keys(state.scope ?? {}).length > 0) && (
        <Paper withBorder p="xs" radius="md">
          <Text size="xs" fw={600} mb={4}>What I know</Text>
          <Group gap={6}>
            {factEntries.map(([k, f]) => (
              <Tooltip key={k} label={`${SOURCE_LABEL[f.source]} · click to change`} withArrow>
                <Badge
                  variant="light"
                  color={SOURCE_COLOR[f.source]}
                  style={{ cursor: busy ? "default" : "pointer", textTransform: "none" }}
                  rightSection={<IconPencil size={10} />}
                  onClick={() => !busy && setText(`Change ${FACT_LABEL[k] ?? k} to `)}
                >
                  {FACT_LABEL[k] ?? k}: {String(f.value)}
                </Badge>
              </Tooltip>
            ))}
            {rooms.map((r) => {
              const sc = state.scope?.[r.id];
              return sc ? (
                <Badge key={r.id} variant="outline" color={SOURCE_COLOR[sc.source]} style={{ textTransform: "none" }}>
                  {r.name}: {sc.state.replace("_", " ")}
                </Badge>
              ) : null;
            })}
          </Group>
        </Paper>
      )}

      {result && <ResultCard result={result} canOverride={!busy} onRate={(code, v) => void send("", { [code]: v })} stale={stale} version={session.blueprint_version} current={currentVersion} />}
    </Stack>
  );
}

function Bubble({ role, text }: { role: "user" | "assistant"; text: string }) {
  return (
    <Paper withBorder p="xs" radius="md" ml={role === "user" ? 24 : 0} mr={role === "assistant" ? 24 : 0} bg={role === "user" ? "var(--mantine-color-blue-light)" : undefined}>
      <Text size="sm" style={{ whiteSpace: "pre-wrap" }}>{text}</Text>
    </Paper>
  );
}

const CONF_COLOR: Record<string, string> = { measured: "teal", derived: "blue", assumed: "orange" };

function ResultCard({ result, canOverride, onRate, stale, version, current }: { result: EstimateResult; canOverride: boolean; onRate: (code: string, v: number) => void; stale: boolean; version: number; current: number }) {
  const rateOf = (code: string) => result.rates.find((r) => r.code === code);
  return (
    <Paper withBorder p="sm" radius="md">
      <Group justify="space-between" mb={4}>
        <Text fw={600} size="sm">Estimate</Text>
        {stale && <Badge color="yellow" variant="light">plan v{version}, now v{current}</Badge>}
      </Group>
      <Text size="xl" fw={700}>{money(result.total.base)} <Text span size="sm" c="dimmed">{result.currency}</Text></Text>
      <Text size="xs" c="dimmed">Range {money(result.total.low)} to {money(result.total.high)} · VAT included</Text>

      <Table mt="xs" fz="xs" withRowBorders={false}>
        <Table.Tbody>
          {Object.entries(result.packages).map(([name, v]) => (
            <Table.Tr key={name}>
              <Table.Td>{name}</Table.Td>
              <Table.Td ta="right">{money(v.base)}</Table.Td>
            </Table.Tr>
          ))}
          <Table.Tr>
            <Table.Td fw={600}>Direct cost</Table.Td>
            <Table.Td ta="right" fw={600}>{money(result.direct.base)}</Table.Td>
          </Table.Tr>
          {result.markups.map((m) => (
            <Table.Tr key={m.name}>
              <Table.Td>
                <Tooltip label={`${m.percent}% of ${m.base_label}`} withArrow><span>{m.name} {m.percent}%</span></Tooltip>
              </Table.Td>
              <Table.Td ta="right">{money(m.base)}</Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>

      {(result.assumptions.length > 0 || result.warnings.length > 0) && (
        <Alert mt="xs" color={result.warnings.length ? "orange" : "gray"} p="xs" title="Assumptions and warnings">
          <Stack gap={2}>
            {[...result.assumptions, ...result.warnings].map((w, i) => (
              <Text key={i} size="xs">{w}</Text>
            ))}
          </Stack>
        </Alert>
      )}

      <Accordion mt="xs" variant="contained" chevronPosition="left">
        <Accordion.Item value="lines">
          <Accordion.Control>
            <Text size="xs" fw={500}>Lines, rates and sources ({result.lines.length})</Text>
          </Accordion.Control>
          <Accordion.Panel>
            <Stack gap="sm">
              {result.lines.map((ln) => (
                <LineRow key={ln.code} line={ln} rate={rateOf(ln.code)} canOverride={canOverride} onRate={onRate} />
              ))}
            </Stack>
          </Accordion.Panel>
        </Accordion.Item>
      </Accordion>
    </Paper>
  );
}

function LineRow({ line, rate, canOverride, onRate }: { line: EstimateLine; rate?: EstimateRate; canOverride: boolean; onRate: (code: string, v: number) => void }) {
  const [value, setValue] = useState<number | string>("");
  return (
    <div>
      <Group justify="space-between" wrap="nowrap" gap="xs">
        <Text size="xs" fw={500}>{line.description.split(",")[0]}</Text>
        <Badge size="xs" variant="light" color={CONF_COLOR[line.confidence]}>{line.confidence}</Badge>
      </Group>
      <Text size="xs" c="dimmed">
        {line.quantity.toLocaleString("vi-VN", { maximumFractionDigits: 2 })} {line.unit}
        {line.priced ? ` × ${money(line.rate_base)} = ${money(line.base)} VND` : " · not priced, not in the total"}
        {line.rate_confidence === "low_confidence" ? " · low confidence" : ""}
      </Text>
      <Text size="xs" c="dimmed">{line.basis.length > 160 ? `${line.basis.slice(0, 160)}…` : line.basis}</Text>
      {rate?.sources.map((s, i) => {
        const href = safeUrl(s.url);
        return (
          <Text key={i} size="xs" c="dimmed" lineClamp={2}>
            {href ? <Anchor href={href} target="_blank" rel="noreferrer noopener" size="xs">{new URL(href).hostname}</Anchor> : "you"}: “{s.quote}” {s.retrieved_at && `(${s.retrieved_at})`}
          </Text>
        );
      })}
      {rate && !line.priced && rate.note && <Text size="xs" c="orange">{rate.note}</Text>}
      {canOverride && (
        <Group gap={4} mt={4} wrap="nowrap">
          <NumberInput size="xs" w={140} min={1} hideControls placeholder={`Your rate / ${line.unit}`} value={value} onChange={setValue} thousandSeparator="," />
          <Button size="compact-xs" variant="light" disabled={typeof value !== "number" || value <= 0} onClick={() => { if (typeof value === "number") { onRate(line.code, value); setValue(""); } }}>
            Use my rate
          </Button>
        </Group>
      )}
    </div>
  );
}
