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

import { AuthDivider, AuthShell, defaultRegisterContent, SocialGoogleButton } from "./auth"

const signUpSchema = z.object({
  name: z.string().min(2, "Enter your full name"),
  email: z.string().min(1, "Email is required").email("Enter a valid email"),
  password: z.string().min(8, "Password must be at least 8 characters"),
})

function zodFieldValidator<T>(schema: z.ZodType<T>) {
  return ({ value }: { value: unknown }) => {
    const result = schema.safeParse(value)
    return result.success ? undefined : (result.error.issues[0]?.message ?? "Invalid value")
  }
}

export function SignUpForm() {
  const router = useRouter()
  const { setAccessToken } = useAuth()
  const [submitError, setSubmitError] = useState<string | null>(null)

  const form = useForm({
    defaultValues: {
      name: "",
      email: "",
      password: "",
    },
    onSubmit: async ({ value }) => {
      setSubmitError(null)
      try {
        const response = await authBrowserRpcClient.register(value)
        await setAuthCookies(response.refreshToken)
        setAccessToken(response.accessToken)
        router.replace("/workspace")
        router.refresh()
      } catch (error) {
        setSubmitError(error instanceof Error ? error.message : "Failed to create account")
        throw error
      }
    },
  })

  const { canSubmit, isSubmitting } = useStore(form.store, (state) => ({
    canSubmit: state.canSubmit,
    isSubmitting: state.isSubmitting,
  }))

  return (
    <AuthShell
      title="Create your account"
      subtitle="Enter your details below to sign up"
      hero={defaultRegisterContent}
      onSubmit={(event) => {
        event.preventDefault()
        event.stopPropagation()
        void form.handleSubmit()
      }}
    >
      <form.Field
        name="name"
        validators={{
          onBlur: zodFieldValidator(signUpSchema.shape.name),
          onChange: zodFieldValidator(signUpSchema.shape.name),
        }}
      >
        {(field) => (
          <div className="space-y-2">
            <Label htmlFor="register-name" className="text-sm font-semibold">
              Full Name
            </Label>
            <Input
              id="register-name"
              name={field.name}
              type="text"
              value={field.state.value}
              onBlur={field.handleBlur}
              onChange={(event) => field.handleChange(event.target.value)}
              placeholder="John Doe"
              autoComplete="name"
            />
            {field.state.meta.errors.length > 0 ? (
              <p className="text-xs text-destructive">{String(field.state.meta.errors[0])}</p>
            ) : null}
          </div>
        )}
      </form.Field>

      <form.Field
        name="email"
        validators={{
          onBlur: zodFieldValidator(signUpSchema.shape.email),
          onChange: zodFieldValidator(signUpSchema.shape.email),
        }}
      >
        {(field) => (
          <div className="space-y-2">
            <Label htmlFor="register-email" className="text-sm font-semibold">
              Email
            </Label>
            <Input
              id="register-email"
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
          onBlur: zodFieldValidator(signUpSchema.shape.password),
          onChange: zodFieldValidator(signUpSchema.shape.password),
        }}
      >
        {(field) => (
          <div className="space-y-2">
            <Label htmlFor="register-password" className="text-sm font-semibold">
              Password
            </Label>
            <Input
              id="register-password"
              name={field.name}
              type="password"
              value={field.state.value}
              onBlur={field.handleBlur}
              onChange={(event) => field.handleChange(event.target.value)}
              placeholder="Password"
              autoComplete="new-password"
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
          className="h-11 w-full border-border bg-white text-sm font-medium text-foreground shadow-sm hover:bg-accent/40 disabled:opacity-70 sm:h-12"
        >
          {isSubmitting ? "Creating account..." : "Create Account"}
        </Button>

        {submitError ? (
          <p className="text-center text-sm text-destructive" role="alert">
            {submitError}
          </p>
        ) : null}

        <p className="text-center text-sm">
          Already have an account?{" "}
          <Link href="/login" className="font-semibold text-foreground">
            Sign in
          </Link>
        </p>

        <AuthDivider />

        <SocialGoogleButton />
      </div>
    </AuthShell>
  )
}
