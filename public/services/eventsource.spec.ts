import { createEventSource } from "./eventsource"
import { http } from "./http"

jest.mock("./http", () => ({ http: { post: jest.fn(), delete: jest.fn() } }))

test.each([1, 2])("reconnecting keeps ownership with the %i subscribed components", (consumers) => {
  jest.useFakeTimers()
  const original = global.EventSource
  const connections: Array<{ readyState: number; onerror: (() => void) | null; onopen: (() => void) | null; close: jest.Mock }> = []
  const constructor = jest.fn(() => {
    const connection = { readyState: 1, onerror: null, onopen: null, close: jest.fn() }
    connections.push(connection)
    return connection
  })
  global.EventSource = Object.assign(constructor, { OPEN: 1, CLOSED: 2 }) as unknown as typeof EventSource

  try {
    const source = createEventSource({ endpoint: "/api/mod/queue-events" })
    for (let i = 0; i < consumers; i++) {
      source.connect()
    }
    expect(connections).toHaveLength(1)

    connections[0].readyState = EventSource.CLOSED
    connections[0].onerror!()
    jest.advanceTimersByTime(1000)
    expect(connections).toHaveLength(2)
    connections[1].onopen!()
    expect(source.isConnected()).toBe(true)

    for (let i = 1; i < consumers; i++) {
      source.disconnect()
      expect(connections[1].close).not.toHaveBeenCalled()
    }

    source.disconnect()
    expect(connections[1].close).toHaveBeenCalledTimes(1)
    expect(source.isConnected()).toBe(false)

    jest.runOnlyPendingTimers()
    expect(connections).toHaveLength(2)
  } finally {
    global.EventSource = original
    jest.useRealTimers()
  }
})

test("presence waits for its own stream identity and replaces it after reconnect", () => {
  jest.useFakeTimers()
  const original = global.EventSource
  const connections: Array<{
    readyState: number
    onerror: (() => void) | null
    onopen: (() => void) | null
    onmessage: ((event: { data: string }) => void) | null
    close: jest.Mock
  }> = []
  const constructor = jest.fn(() => {
    const connection = { readyState: 1, onerror: null, onopen: null, onmessage: null, close: jest.fn() }
    connections.push(connection)
    return connection
  })
  global.EventSource = Object.assign(constructor, { OPEN: 1, CLOSED: 2 }) as unknown as typeof EventSource
  jest.mocked(http.post).mockResolvedValue({ ok: true, data: {} })
  jest.mocked(http.delete).mockResolvedValue({ ok: true, data: undefined })

  const first = createEventSource({
    endpoint: "/queue-events",
    heartbeatConfig: { heartbeatEndpoint: id => `/queue/${id}`, stopViewingEndpoint: "/queue-viewing" },
  })
  const second = createEventSource({
    endpoint: "/queue-events",
    heartbeatConfig: { heartbeatEndpoint: id => `/queue/${id}`, stopViewingEndpoint: "/queue-viewing" },
  })

  try {
    first.connect()
    second.connect()
    first.viewItem(100)
    second.viewItem(200)
    connections[0].onopen!()
    expect(http.post).not.toHaveBeenCalled()

    connections[0].onmessage!({ data: JSON.stringify({ type: "connection.ready", payload: { connectionId: "first" } }) })
    connections[1].onmessage!({ data: JSON.stringify({ type: "connection.ready", payload: { connectionId: "second" } }) })
    expect(http.post).toHaveBeenNthCalledWith(1, "/queue/100", { connectionId: "first" }, { notifyOnError: false })
    expect(http.post).toHaveBeenNthCalledWith(2, "/queue/200", { connectionId: "second" }, { notifyOnError: false })

    connections[0].readyState = 0
    connections[0].onerror!()
    first.viewItem(300)
    expect(http.post).toHaveBeenCalledTimes(2)
    connections[0].readyState = 1
    connections[0].onopen!()
    expect(http.post).toHaveBeenCalledTimes(2)

    connections[0].onmessage!({ data: JSON.stringify({ type: "connection.ready", payload: { connectionId: "replacement" } }) })
    expect(http.post).toHaveBeenLastCalledWith("/queue/300", { connectionId: "replacement" }, { notifyOnError: false })
    first.stopViewing()
    expect(http.delete).toHaveBeenLastCalledWith("/queue-viewing", { connectionId: "replacement" }, { notifyOnError: false })
    second.stopViewing()
    expect(http.delete).toHaveBeenLastCalledWith("/queue-viewing", { connectionId: "second" }, { notifyOnError: false })
  } finally {
    first.disconnect()
    second.disconnect()
    global.EventSource = original
    jest.clearAllMocks()
    jest.useRealTimers()
  }
})
