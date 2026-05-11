import * as React from "react"

import { cn } from "@/lib/utils"

function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        "h-9 w-full min-w-0 rounded-md border border-hairline bg-canvas px-3 py-2 text-base text-ink placeholder:text-stone transition-colors outline-none file:inline-flex file:h-6 file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground focus-visible:border-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:pointer-events-none disabled:cursor-not-allowed disabled:bg-surface-soft disabled:opacity-50 aria-invalid:border-semantic-error aria-invalid:focus-visible:outline-semantic-error sm:text-sm",
        className,
      )}
      {...props}
    />
  )
}

export { Input }
