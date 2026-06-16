export type PollOption = {
  id: string
  text: string
  votes: string[]
}

export type PollProps = {
  question: string
  options: PollOption[]
}

export type PollSlateElement = {
  id: string
  type: "poll"
  props: PollProps
  children: [{ text: "" }]
}
