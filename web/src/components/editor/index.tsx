"use client"

import { useCallback, useEffect, useMemo, useRef } from "react"
import YooptaEditor, {
  createYooptaEditor,
  type YooEditor,
  RenderBlockProps,
  SlateElement,
  YooptaContentValue,
  YooptaPlugin,
} from "@yoopta/editor"
import { SelectionBox } from "@yoopta/ui/selection-box"
import { BlockDndContext, SortableBlock } from "@yoopta/ui/block-dnd"
import { withMentions } from "@yoopta/mention"
// @ts-expect-error — MentionDropdown types not exported
import { MentionDropdown } from "@yoopta/themes-shadcn/mention"
// @ts-expect-error — EmojiDropdown types not exported
import { EmojiDropdown } from "@yoopta/themes-shadcn/emoji"
import { applyTheme } from "@yoopta/themes-shadcn"
import { withEmoji } from "@yoopta/emoji"

import { PollUserProvider, YOOPTA_PLUGINS } from "./plugins"
import { YOOPTA_MARKS } from "./marks"
import { YooptaToolbar } from "./new-yoo-components/yoopta-toolbar"
import { YooptaSlashCommandMenu } from "./new-yoo-components/yoopta-slash-command-menu"
import { YooptaFloatingBlockActions } from "./new-yoo-components/yoopta-floating-block-actions"

// Computed once at module load — never re-created on component mount.
const THEMED_PLUGINS = applyTheme(YOOPTA_PLUGINS) as unknown as YooptaPlugin<
  Record<string, SlateElement>,
  unknown
>[]

const EDITOR_STYLES = { width: "100%", paddingBottom: 100 }

export type FullSetupEditorProps = {
  /** Called on every content change with the full Yoopta block map. */
  onChange?: (value: YooptaContentValue) => void
  /**
   * Ref that receives the live YooEditor instance on mount.
   * NoteView uses this to call editor.setEditorValue() when remote updates arrive.
   */
  editorRef?: React.RefObject<YooEditor | null>
  /**
   * Callback fired once the editor instance is ready (after the container div mounts).
   * More reliable than editorRef for running logic that depends on the editor being ready.
   */
  onEditorReady?: (editor: YooEditor | null) => void
  /** Optional ref to the container div (forwarded to SelectionBox). */
  containerBoxRef?: React.RefObject<HTMLDivElement>
}

export function FullSetupEditor({
  onChange,
  editorRef,
  onEditorReady,
  containerBoxRef: externalRef,
}: FullSetupEditorProps) {
  const internalRef = useRef<HTMLDivElement>(null)

  const editor = useMemo(
    () =>
      withEmoji(
        withMentions(
          createYooptaEditor({ plugins: THEMED_PLUGINS, marks: YOOPTA_MARKS }),
        ),
      ),
    [],
  )

  // Forward the editor instance to the caller via ref and callback after mount.
  const setContainerRef = useCallback(
    (node: HTMLDivElement | null) => {
      // Always write both the internal ref and the external ref if provided.
      ;(internalRef as React.MutableRefObject<HTMLDivElement | null>).current = node
      if (externalRef) {
        ;(externalRef as React.MutableRefObject<HTMLDivElement | null>).current = node
      }

      // Always update editorRef regardless of node presence.
      if (editorRef) {
        ;(editorRef as React.MutableRefObject<YooEditor | null>).current = node ? editor : null
      }

      // Fire the ready callback — this is the reliable signal.
      onEditorReady?.(node ? editor : null)
    },
    [editor, editorRef, externalRef, onEditorReady],
  )

  const renderBlock = useCallback(
    ({ children, blockId }: RenderBlockProps) => (
      <SortableBlock id={blockId} useDragHandle>
        {children}
      </SortableBlock>
    ),
    [],
  )

  return (
    <PollUserProvider>
      {/*
       * .yoopta-theme-scope scopes HSL CSS variables for @yoopta/themes-shadcn.
       * The rest of the app defines variables as hex; Yoopta expects bare HSL
       * triplets (e.g. hsl(var(--background))). This wrapper keeps the fix
       * contained so nothing outside the editor is affected.
       *
       * overflow-visible ensures descenders on large Yoopta headings (g, y, p…)
       * are not clipped by a containing element with overflow:hidden.
       */}
      <div
        ref={setContainerRef}
        className="yoopta-theme-scope w-full max-w-4xl mx-auto overflow-visible"
      >
        <BlockDndContext editor={editor}>
          <YooptaEditor
            editor={editor}
            style={EDITOR_STYLES}
            renderBlock={renderBlock}
            onChange={onChange}
            placeholder="Type / to open menu, or start typing…"
          >
            <YooptaToolbar />
            <YooptaFloatingBlockActions />
            <YooptaSlashCommandMenu />
            <SelectionBox selectionBoxElement={internalRef} />
            <MentionDropdown />
            <EmojiDropdown />
          </YooptaEditor>
        </BlockDndContext>
      </div>
    </PollUserProvider>
  )
}
