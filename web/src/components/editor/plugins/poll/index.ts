import { YooptaPlugin } from "@yoopta/editor"
import { PollComponent } from "./poll-component"

export const PollPlugin = new YooptaPlugin({
  type: "Poll",
  elements: {
    poll: {
      render: PollComponent,
      props: {
        question: "Your question here…",
        options: [
          { id: "default-opt-1", text: "", votes: [] },
          { id: "default-opt-2", text: "", votes: [] },
        ],
        nodeType: "void",
      },
    },
  },
  options: {
    display: {
      title: "Poll",
      description: "Create a multiple-choice poll",
      icon: "📊",
    },
    shortcuts: ["poll"],
  },
  lifecycle: {
    beforeCreate: (editor) => {
      return editor.y("poll", {
        props: {
          question: "Your question here…",
          options: [
            { id: crypto.randomUUID(), text: "", votes: [] },
            { id: crypto.randomUUID(), text: "", votes: [] },
          ],
          nodeType: "void",
        },
        children: [editor.y.text("")],
      })
    },
  },
  parsers: {
    html: {
      serialize: (element, _text) => {
        const { question, options } = element.props as {
          question: string
          options: Array<{ id: string; text: string; votes: string[] }>
        }

        const items = options
          .map(
            (o) =>
              `<li class="yoopta-poll__option">${o.text || "Untitled option"} <span class="yoopta-poll__votes">(${o.votes.length} ${o.votes.length === 1 ? "vote" : "votes"})</span></li>`,
          )
          .join("")

        return (
          `<div class="yoopta-poll">` +
          `<h3 class="yoopta-poll__question">${question}</h3>` +
          `<ul class="yoopta-poll__options">${items}</ul>` +
          `</div>`
        )
      },
      deserialize: {
        nodeNames: ["DIV"],
        parse: (el, editor) => {
          if (!el.classList.contains("yoopta-poll")) return undefined

          const questionEl = el.querySelector(".yoopta-poll__question")
          const optionEls = Array.from(el.querySelectorAll(".yoopta-poll__option"))

          return editor.y("poll", {
            props: {
              question: questionEl?.textContent?.trim() ?? "",
              options: optionEls.map((li, i) => ({
                id: crypto.randomUUID(),
                text: li.firstChild?.textContent?.trim() ?? `Option ${i + 1}`,
                votes: [] as string[],
              })),
              nodeType: "void",
            },
            children: [editor.y.text("")],
          })
        },
      },
    },
    markdown: {
      serialize: (element, _text) => {
        const { question, options } = element.props as {
          question: string
          options: Array<{ text: string; votes: string[] }>
        }

        const lines = options.map((o) => `- [ ] ${o.text || "Untitled option"}`).join("\n")

        return `**Poll: ${question}**\n\n${lines}\n`
      },
    },
  },
})

export { PollUserProvider, usePollUser } from "./poll-user-context"
export type { PollOption, PollProps, PollSlateElement } from "./types"
