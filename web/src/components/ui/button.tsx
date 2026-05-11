import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { Slot } from "radix-ui"

import { cn } from "@/lib/utils"

const buttonVariants = cva(
  "group/button inline-flex shrink-0 items-center justify-center rounded-md border font-medium whitespace-nowrap transition-colors outline-none select-none focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default:
          "bg-primary text-on-primary hover:bg-primary-pressed active:bg-primary-deep disabled:bg-hairline disabled:text-muted",
        outline:
          "border-hairline bg-canvas text-ink hover:bg-surface-soft active:bg-surface disabled:border-hairline-soft disabled:text-muted",
        secondary:
          "bg-secondary text-ink hover:bg-slate hover:text-ink active:bg-charcoal disabled:bg-surface-soft",
        ghost:
          "text-ink hover:bg-surface-soft active:bg-surface disabled:text-muted",
        destructive:
          "bg-semantic-error text-white hover:bg-semantic-error/80 active:bg-semantic-error/70",
        link: "text-link-blue underline-offset-4 hover:text-link-blue-pressed hover:underline active:text-link-blue-pressed",
      },
      size: {
        default: "h-10 gap-2 px-4 py-2.5 text-sm",
        xs: "h-7 gap-1 px-2 text-xs rounded-sm",
        sm: "h-8 gap-1.5 px-3 text-sm rounded-sm",
        lg: "h-11 gap-2.5 px-6 py-3 text-base",
        icon: "size-10",
        "icon-xs": "size-7 rounded-sm",
        "icon-sm": "size-8 rounded-sm",
        "icon-lg": "size-11",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  },
)

function Button({
  className,
  variant = "default",
  size = "default",
  asChild = false,
  ...props
}: React.ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean
  }) {
  const Comp = asChild ? Slot.Root : "button"

  return (
    <Comp
      data-slot="button"
      data-variant={variant}
      data-size={size}
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  )
}

export { Button, buttonVariants }
