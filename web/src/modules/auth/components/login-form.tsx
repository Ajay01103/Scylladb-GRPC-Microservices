"use client"

import Link from "next/link"
import { useRouter } from "next/navigation"
import { useState } from "react"
import { useForm } from "@tanstack/react-form"
import { useStore } from "@tanstack/react-form"
import { z } from "zod"

import { setAuthCookies } from "@/actions/auth"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useAuth } from "@/lib/auth-context"
import { authBrowserRpcClient } from "@/lib/rpc"

import { AuthDivider, AuthShell, defaultLoginContent, SocialGoogleButton } from "./auth"

const loginSchema = z.object({
  email: z.string().min(1, "Email is required").email("Enter a valid email"),
  password: z.string().min(1, "Password is required"),
})

/**
 * Wrap a Zod field schema into the `validators.onChange` / `validators.onBlur`
 * shape TanStack Form expects. Returns the first issue's message, or
 * `undefined` when valid — per the skill's rule "Return `undefined` (not
 * `null`/`false`) for valid fields".
 *
 * Shared between login + sign-up so the behavior stays consistent.
 */
function zodFieldValidator<T>(schema: z.ZodType<T>) {
  return ({ value }: { value: unknown }) => {
    const result = schema.safeParse(value)
    return result.success ? undefined : (result.error.issues[0]?.message ?? "Invalid value")
  }
}

export function LoginForm() {
  const router = useRouter()
  const { setAccessToken } = useAuth()
  // `useState` (not `useRef`) — refs don't trigger re-renders, so the
  // submit error would never display. This was a real UX bug.
  const [submitError, setSubmitError] = useState<string | null>(null)

  const form = useForm({
    defaultValues: {
      email: "",
      password: "",
    },
    onSubmit: async ({ value }) => {
      setSubmitError(null)
      try {
        const response = await authBrowserRpcClient.login(value)
        await setAuthCookies(response.refreshToken)
        setAccessToken(response.accessToken)
        router.replace("/workspace")
        router.refresh()
      } catch (error) {
        setSubmitError(error instanceof Error ? error.message : "Failed to sign in")
        // Re-throw so TanStack Form marks `isSubmitSuccessful = false` and
        // surfaces the error in `state.errors` for any subscribed banner.
        throw error
      }
    },
  })

  // canSubmit = isValid && !isSubmitting — disables the button when the form
  // has invalid fields too, not just during in-flight requests.
  const { canSubmit, isSubmitting } = useStore(form.store, (state) => ({
    canSubmit: state.canSubmit,
    isSubmitting: state.isSubmitting,
  }))

  return (
    <AuthShell
      title="Sign in to your account"
      subtitle="Enter your email below to sign in"
      hero={defaultLoginContent}
      onSubmit={(event) => {
        event.preventDefault()
        event.stopPropagation()
        void form.handleSubmit()
      }}
    >
      <form.Field
        name="email"
        validators={{
          onBlur: zodFieldValidator(loginSchema.shape.email),
          onChange: zodFieldValidator(loginSchema.shape.email),
        }}
      >
        {(field) => (
          <div className="space-y-2">
            <Label htmlFor="login-email" className="text-sm font-semibold">
              Email
            </Label>
            <Input
              id="login-email"
              name={field.name}
              type="email"
              value={field.state.value}
              onBlur={field.handleBlur}
              onChange={(event) => field.handleChange(event.target.value)}
              placeholder="m@example.com"
              autoComplete="email"
            />
            {field.state.meta.errors.length > 0 ? (
              <p className="text-xs text-destructive">{String(field.state.meta.errors[0])}</p>
            ) : null}
          </div>
        )}
      </form.Field>

      <form.Field
        name="password"
        validators={{
          onBlur: zodFieldValidator(loginSchema.shape.password),
          onChange: zodFieldValidator(loginSchema.shape.password),
        }}
      >
        {(field) => (
          <div className="space-y-2">
            <Label htmlFor="login-password" className="text-sm font-semibold">
              Password
            </Label>
            <Input
              id="login-password"
              name={field.name}
              type="password"
              value={field.state.value}
              onBlur={field.handleBlur}
              onChange={(event) => field.handleChange(event.target.value)}
              placeholder="Password"
              autoComplete="current-password"
            />
            {field.state.meta.errors.length > 0 ? (
              <p className="text-xs text-destructive">{String(field.state.meta.errors[0])}</p>
            ) : null}
          </div>
        )}
      </form.Field>

      <div className="space-y-4">
        <Button
          type="submit"
          variant="outline"
          disabled={!canSubmit}
          className="h-11 w-full border-border text-foreground shadow-sm hover:bg-accent/40 disabled:opacity-70 sm:h-12"
        >
          {isSubmitting ? "Signing in..." : "Sign In"}
        </Button>

        {submitError ? (
          <p className="text-center text-sm text-destructive" role="alert">
            {submitError}
          </p>
        ) : null}

        <p className="text-center text-sm">
          Don't have an account?{" "}
          <Link href="/register" className="font-semibold text-foreground">
            Sign up
          </Link>
        </p>

        <AuthDivider />

        <SocialGoogleButton />
      </div>
    </AuthShell>
  )
}
