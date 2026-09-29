import React, { useEffect, useRef } from "react"
import * as Y from "yjs"
import { Awareness } from "y-protocols/awareness"
import { EditorSelection, EditorState, StateEffect } from "@codemirror/state"
import { EditorView, keymap, layer, LayerMarker, placeholder, RectangleMarker } from "@codemirror/view"
import { defaultKeymap } from "@codemirror/commands"
import { markdown } from "@codemirror/lang-markdown"
import { yCollab, yUndoManagerKeymap } from "y-codemirror.next"

interface Props {
  document: Y.Doc
  awareness: Awareness
  scrollElement: React.MutableRefObject<HTMLElement | null>
  onScroll: () => void
}

interface PeerMarker extends LayerMarker {
  rectangle: RectangleMarker
  color: string
  name?: string
}

function peerMarker(rectangle: RectangleMarker, color: string, name?: string): PeerMarker {
  return {
    rectangle,
    color,
    name,
    eq(marker) {
      const other = marker as PeerMarker
      return rectangle.eq(other.rectangle) && color === other.color && name === other.name
    },
    draw() {
      const element = rectangle.draw()
      element.style.backgroundColor = color
      if (name !== undefined) {
        element.title = name
      }

      return element
    },
  }
}

export default function CollaborativePageContent(props: Props) {
  const host = useRef<HTMLDivElement>(null)
  const scroll = useRef(props.onScroll)
  scroll.current = props.onScroll

  useEffect(() => {
    const text = props.document.getText("content")
    const undoManager = new Y.UndoManager(text)
    const peersChanged = StateEffect.define<void>()
    const view = new EditorView({
      parent: host.current!,
      state: EditorState.create({
        doc: text.toString(),
        extensions: [
          markdown(),
          EditorView.lineWrapping,
          keymap.of([...yUndoManagerKeymap, ...defaultKeymap]),
          placeholder("Write your page content in Markdown..."),
          yCollab(text, null, { undoManager }),
          EditorView.updateListener.of(update => {
            if (!update.selectionSet && !update.docChanged && !update.focusChanged) {
              return
            }

            const selection = update.state.selection.main
            const cursor = update.view.hasFocus ? {
              anchor: Y.createRelativePositionFromTypeIndex(text, selection.anchor),
              head: Y.createRelativePositionFromTypeIndex(text, selection.head),
            } : null
            const previous = props.awareness.getLocalState()?.cursor
            if (cursor && previous && Y.compareRelativePositions(cursor.anchor, previous.anchor) && Y.compareRelativePositions(cursor.head, previous.head)) {
              return
            }

            if (cursor || previous) {
              props.awareness.setLocalStateField("cursor", cursor)
            }
          }),
          // Peer presence must not change text layout or cursor navigation.
          layer({
            above: true,
            update(update) {
              return update.docChanged || update.viewportChanged || update.transactions.some(transaction =>
                transaction.effects.some(effect => effect.is(peersChanged)))
            },
            markers(current) {
              const markers: PeerMarker[] = []
              for (const [clientID, state] of props.awareness.getStates()) {
                if (clientID === props.document.clientID || !state.cursor) {
                  continue
                }

                const anchor = Y.createAbsolutePositionFromRelativePosition(state.cursor.anchor, props.document)
                const head = Y.createAbsolutePositionFromRelativePosition(state.cursor.head, props.document)
                if (!anchor || !head || anchor.type !== text || head.type !== text) {
                  continue
                }

                if (anchor.index !== head.index) {
                  const range = EditorSelection.range(anchor.index, head.index)
                  for (const rectangle of RectangleMarker.forRange(current, "cm-peer-selection", range)) {
                    markers.push(peerMarker(rectangle, state.user.colorLight))
                  }
                }

                const cursor = EditorSelection.cursor(head.index, state.cursor.head.assoc)
                for (const rectangle of RectangleMarker.forRange(current, "cm-peer-cursor", cursor)) {
                  markers.push(peerMarker(rectangle, state.user.color, state.user.name))
                }
              }

              return markers
            },
          }),
          EditorView.contentAttributes.of({ "aria-label": "Page content", role: "textbox", "aria-multiline": "true" }),
          EditorView.domEventHandlers({ scroll: () => scroll.current() }),
          EditorView.theme({
            "&": { height: "100%", backgroundColor: "var(--color-surface)", color: "var(--color-foreground)" },
            ".cm-scroller": { overflow: "auto", fontFamily: "monospace", fontSize: "0.875rem" },
            ".cm-content": { padding: "0.75rem", minHeight: "200px", caretColor: "var(--color-foreground)" },
            "&.cm-focused": { outline: "2px solid var(--color-primary)", outlineOffset: "-2px" },
            ".cm-peer-cursor": { width: "2px", pointerEvents: "auto" },
            ".cm-peer-cursor::before": {
              content: "''",
              position: "absolute",
              width: "6px",
              height: "6px",
              top: "-3px",
              left: "-2px",
              borderRadius: "50%",
              backgroundColor: "inherit",
            },
          }),
        ],
      }),
    })
    props.scrollElement.current = view.scrollDOM

    const refreshPeers = (change: { added: number[]; updated: number[]; removed: number[] }) => {
      const isPeer = (clientID: number) => clientID !== props.document.clientID
      if (change.added.some(isPeer) || change.updated.some(isPeer) || change.removed.some(isPeer)) {
        view.dispatch({ effects: peersChanged.of() })
      }
    }
    props.awareness.on("change", refreshPeers)

    return () => {
      props.awareness.off("change", refreshPeers)
      props.scrollElement.current = null
      view.destroy()
      undoManager.destroy()
    }
  }, [props.document, props.awareness])

  return <div ref={host} className="min-h-[200px] flex-1 overflow-hidden rounded-card border border-border" />
}
