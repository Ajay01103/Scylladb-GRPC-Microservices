"use client"

import type { ChangeEvent, MouseEvent } from "react"
import { BarChart2, Check, X } from "lucide-react"
import {
  Elements,
  useYooptaEditor,
  type PluginElementRenderProps,
  type YooEditor,
} from "@yoopta/editor"
import type { PollOption, PollProps } from "./types"
import { usePollUser } from "./poll-user-context"
import "./poll.css"

export function PollComponent({
  element,
  attributes,
  children,
  blockId,
}: PluginElementRenderProps) {
  const editor: YooEditor = useYooptaEditor()
  const { currentUser, isAuthenticated } = usePollUser()
  const { question, options } = element.props as PollProps

  const totalVotes = options.reduce((sum, o) => sum + o.votes.length, 0)

  const updateProps = (patch: Partial<PollProps>) => {
    Elements.updateElement(editor, {
      blockId,
      type: "poll",
      props: patch,
    })
  }

  const handleVote = (option: PollOption) => {
    if (!isAuthenticated) return

    const alreadyVoted = option.votes.includes(currentUser)
    const updatedOptions = options.map((o) =>
      o.id !== option.id
        ? o
        : {
            ...o,
            votes: alreadyVoted
              ? o.votes.filter((v) => v !== currentUser)
              : [...o.votes, currentUser],
          },
    )
    updateProps({ options: updatedOptions })
  }

  const handleOptionText = (option: PollOption, text: string) => {
    updateProps({
      options: options.map((o) => (o.id === option.id ? { ...o, text } : o)),
    })
  }

  const handleAddOption = () => {
    updateProps({
      options: [...options, { id: crypto.randomUUID(), text: "", votes: [] }],
    })
  }

  const handleDeleteOption = (event: MouseEvent<HTMLButtonElement>, option: PollOption) => {
    event.stopPropagation()
    if (options.length <= 2) return
    updateProps({ options: options.filter((o) => o.id !== option.id) })
  }

  const handleQuestion = (e: ChangeEvent<HTMLInputElement>) => {
    updateProps({ question: e.target.value })
  }

  const stopEditorPropagation = (event: MouseEvent<HTMLElement>) => {
    event.stopPropagation()
  }

  return (
    <div {...attributes} className="poll-container">
      <span style={{ display: "none" }}>{children}</span>

      <div contentEditable={false} className="poll-inner">
        <div className="poll-label">
          <BarChart2 size={12} aria-hidden="true" />
          <span>Poll</span>
        </div>

        <div className="poll-header">
          <input
            className="poll-question-input"
            value={question}
            onChange={handleQuestion}
            onMouseDown={stopEditorPropagation}
            placeholder="Ask a question…"
            aria-label="Poll question"
          />
        </div>

        <div className="poll-options" role="group" aria-label="Poll options">
          {options.map((option, index) => {
            const votePercent =
              totalVotes === 0 ? 0 : Math.round((option.votes.length / totalVotes) * 100)
            const hasVoted = option.votes.includes(currentUser)
            const showResults = totalVotes > 0

            return (
              <div
                key={option.id}
                className={[
                  "poll-option",
                  hasVoted ? "poll-option--voted" : "",
                  !isAuthenticated ? "poll-option--disabled" : "",
                ]
                  .filter(Boolean)
                  .join(" ")}
                onClick={() => handleVote(option)}
                onKeyDown={(event) => {
                  // Only treat Space/Enter as a vote when the div itself has
                  // keyboard focus (i.e. it's being activated as a button via
                  // the keyboard). If the event bubbled up from the text input
                  // inside, ignore it — the user is just typing a space.
                  if (event.target !== event.currentTarget) return
                  if (event.key === "Enter" || event.key === " ") {
                    event.preventDefault()
                    handleVote(option)
                  }
                }}
                role="button"
                tabIndex={isAuthenticated ? 0 : -1}
                aria-pressed={hasVoted}
              >
                <label
                  className="poll-option-check"
                  htmlFor={`poll-${blockId}-opt-${option.id}`}
                  onClick={(event) => event.stopPropagation()}
                >
                  <input
                    type="checkbox"
                    checked={hasVoted}
                    disabled={!isAuthenticated}
                    onChange={() => handleVote(option)}
                    id={`poll-${blockId}-opt-${option.id}`}
                    aria-label={`Vote for option ${index + 1}`}
                  />
                  <span className="poll-option-checkmark" aria-hidden="true">
                    {hasVoted ? <Check size={10} strokeWidth={2.5} /> : null}
                  </span>
                </label>

                <div className="poll-option-content">
                  {showResults ? (
                    <div
                      className="poll-option-bar"
                      style={{ width: `${votePercent}%` }}
                      aria-hidden="true"
                    />
                  ) : null}

                  <div className="poll-option-row">
                    <input
                      className="poll-option-text"
                      type="text"
                      value={option.text}
                      onChange={(e) => handleOptionText(option, e.target.value)}
                      onMouseDown={stopEditorPropagation}
                      onClick={stopEditorPropagation}
                      placeholder={`Option ${index + 1}`}
                      aria-label={`Option ${index + 1} text`}
                    />

                    <div className="poll-option-meta">
                      {showResults && votePercent > 0 ? (
                        <span className="poll-option-pct">{votePercent}%</span>
                      ) : null}
                      {option.votes.length > 0 ? (
                        <span className="poll-option-votes" aria-live="polite">
                          {option.votes.length} {option.votes.length === 1 ? "vote" : "votes"}
                        </span>
                      ) : null}
                    </div>
                  </div>
                </div>

                <button
                  className="poll-option-delete"
                  onClick={(event) => handleDeleteOption(event, option)}
                  onMouseDown={stopEditorPropagation}
                  disabled={options.length <= 2}
                  aria-label={`Remove option ${index + 1}`}
                  title={options.length <= 2 ? "Need at least 2 options" : "Remove option"}
                  type="button"
                >
                  <X size={14} />
                </button>
              </div>
            )
          })}
        </div>

        <div className="poll-footer">
          <button
            className="poll-add-btn"
            onClick={handleAddOption}
            onMouseDown={stopEditorPropagation}
            type="button"
            aria-label="Add a new poll option"
          >
            Add option
          </button>
          <span className="poll-total-votes" aria-live="polite">
            {totalVotes} {totalVotes === 1 ? "vote" : "votes"} total
            {!isAuthenticated ? <span className="poll-auth-hint"> · Sign in to vote</span> : null}
          </span>
        </div>
      </div>
    </div>
  )
}
