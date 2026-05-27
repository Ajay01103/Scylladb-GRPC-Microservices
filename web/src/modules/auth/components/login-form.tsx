"use client"

import { useRef } from "react"
import { useForm } from "@tanstack/react-form"
import { useStore } from "@tanstack/react-form"
import { useRouter } from "next/navigation"
import { z } from "zod"
import { Label } from "@/components/ui/label"
import { AuthDivider, AuthShell, defaultLoginContent, SocialGoogleButton } from "./auth"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import Link from "next/link"
import { authBrowserRpcClient } from "@/lib/rpc"
import { setAuthCookies } from "@/actions/auth"
import { useAuth } from "@/lib/auth-context"

const loginSchema = z.object({
  email: z.string().min(1, "Email is required").email("Enter a valid email"),
  password: z.string().min(1, "Password is required"),
})

function getFirstErrorMessage(error: unknown) {
  return typeof error === "string" ? error : "Invalid value"
}

export function LoginForm() {
  const router = useRouter()
  const { setAccessToken } = useAuth()
  const submitErrorRef = useRef<string | null>(null)

  const form = useForm({
    defaultValues: {
      email: "",
      password: "",
    },
    onSubmit: async ({ value }) => {
      const parsed = loginSchema.safeParse(value)

      if (!parsed.success) {
        return
      }

      submitErrorRef.current = null

      try {
        const response = await authBrowserRpcClient.login(parsed.data)
        await setAuthCookies(response.refreshToken)
        setAccessToken(response.accessToken)
        router.replace("/workspace")
        router.refresh()
      } catch (error) {
        submitErrorRef.current = error instanceof Error ? error.message : "Failed to sign in"
        throw error
      }
    },
  })

  const isSubmitting = useStore(form.store, (state) => state.isSubmitting)

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
          onBlur: ({ value }) => {
            const result = loginSchema.shape.email.safeParse(value)
            return result.success
              ? undefined
              : getFirstErrorMessage(result.error.issues[0]?.message)
          },
          onChange: ({ value }) => {
            const result = loginSchema.shape.email.safeParse(value)
            return result.success
              ? undefined
              : getFirstErrorMessage(result.error.issues[0]?.message)
          },
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
          onBlur: ({ value }) => {
            const result = loginSchema.shape.password.safeParse(value)
            return result.success
              ? undefined
              : getFirstErrorMessage(result.error.issues[0]?.message)
          },
          onChange: ({ value }) => {
            const result = loginSchema.shape.password.safeParse(value)
            return result.success
              ? undefined
              : getFirstErrorMessage(result.error.issues[0]?.message)
          },
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
          disabled={isSubmitting}
          className="h-11 w-full border-border text-foreground shadow-sm hover:bg-accent/40 disabled:opacity-70 sm:h-12"
        >
          {isSubmitting ? "Signing in..." : "Sign In"}
        </Button>

        {submitErrorRef.current ? (
          <p className="text-center text-sm text-destructive">{submitErrorRef.current}</p>
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
