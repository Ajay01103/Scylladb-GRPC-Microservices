"use client"

import { useRef } from "react"
import { useForm } from "@tanstack/react-form"
import { useStore } from "@tanstack/react-form"
import { useRouter } from "next/navigation"
import { z } from "zod"
import { Label } from "@/components/ui/label"
import {
  AuthDivider,
  AuthShell,
  defaultRegisterContent,
  SocialGoogleButton,
} from "./auth"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import Link from "next/link"
import { authBrowserRpcClient } from "@/lib/rpc"
import { setAuthCookies } from "@/actions/auth"
import { useAuth } from "@/lib/auth-context"

const signUpSchema = z.object({
  name: z.string().min(2, "Enter your full name"),
  email: z.string().min(1, "Email is required").email("Enter a valid email"),
  password: z.string().min(8, "Password must be at least 8 characters"),
})

function getFirstErrorMessage(error: unknown) {
  return typeof error === "string" ? error : "Invalid value"
}

export function SignUpForm() {
  const router = useRouter()
  const { setAccessToken } = useAuth()
  const submitErrorRef = useRef<string | null>(null)

  const form = useForm({
    defaultValues: {
      name: "",
      email: "",
      password: "",
    },
    onSubmit: async ({ value }) => {
      const parsed = signUpSchema.safeParse(value)

      if (!parsed.success) {
        return
      }

      submitErrorRef.current = null

      try {
        const response = await authBrowserRpcClient.register(parsed.data)
        await setAuthCookies(response.refreshToken)
        setAccessToken(response.accessToken)
        router.replace("/")
        router.refresh()
      } catch (error) {
        submitErrorRef.current =
          error instanceof Error ? error.message : "Failed to create account"
        throw error
      }
    },
  })

  const isSubmitting = useStore(form.store, (state) => state.isSubmitting)

  return (
    <AuthShell
      title="Create your account"
      subtitle="Enter your details below to sign up"
      hero={defaultRegisterContent}
      onSubmit={(event) => {
        event.preventDefault()
        event.stopPropagation()
        void form.handleSubmit()
      }}>
      <form.Field
        name="name"
        validators={{
          onBlur: ({ value }) => {
            const result = signUpSchema.shape.name.safeParse(value)
            return result.success
              ? undefined
              : getFirstErrorMessage(result.error.issues[0]?.message)
          },
          onChange: ({ value }) => {
            const result = signUpSchema.shape.name.safeParse(value)
            return result.success
              ? undefined
              : getFirstErrorMessage(result.error.issues[0]?.message)
          },
        }}>
        {(field) => (
          <div className="space-y-2">
            <Label
              htmlFor="register-name"
              className="text-sm font-semibold">
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
              <p className="text-xs text-destructive">
                {String(field.state.meta.errors[0])}
              </p>
            ) : null}
          </div>
        )}
      </form.Field>

      <form.Field
        name="email"
        validators={{
          onBlur: ({ value }) => {
            const result = signUpSchema.shape.email.safeParse(value)
            return result.success
              ? undefined
              : getFirstErrorMessage(result.error.issues[0]?.message)
          },
          onChange: ({ value }) => {
            const result = signUpSchema.shape.email.safeParse(value)
            return result.success
              ? undefined
              : getFirstErrorMessage(result.error.issues[0]?.message)
          },
        }}>
        {(field) => (
          <div className="space-y-2">
            <Label
              htmlFor="register-email"
              className="text-sm font-semibold">
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
              <p className="text-xs text-destructive">
                {String(field.state.meta.errors[0])}
              </p>
            ) : null}
          </div>
        )}
      </form.Field>

      <form.Field
        name="password"
        validators={{
          onBlur: ({ value }) => {
            const result = signUpSchema.shape.password.safeParse(value)
            return result.success
              ? undefined
              : getFirstErrorMessage(result.error.issues[0]?.message)
          },
          onChange: ({ value }) => {
            const result = signUpSchema.shape.password.safeParse(value)
            return result.success
              ? undefined
              : getFirstErrorMessage(result.error.issues[0]?.message)
          },
        }}>
        {(field) => (
          <div className="space-y-2">
            <Label
              htmlFor="register-password"
              className="text-sm font-semibold">
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
              <p className="text-xs text-destructive">
                {String(field.state.meta.errors[0])}
              </p>
            ) : null}
          </div>
        )}
      </form.Field>

      <div className="space-y-4">
        <Button
          type="submit"
          variant="outline"
          disabled={isSubmitting}
          className="h-11 w-full border-border bg-white text-sm font-medium text-foreground shadow-sm hover:bg-accent/40 disabled:opacity-70 sm:h-12">
          {isSubmitting ? "Creating account..." : "Create Account"}
        </Button>

        {submitErrorRef.current ? (
          <p className="text-center text-sm text-destructive">{submitErrorRef.current}</p>
        ) : null}

        <p className="text-center text-sm">
          Already have an account?{" "}
          <Link
            href="/login"
            className="font-semibold text-foreground">
            Sign in
          </Link>
        </p>

        <AuthDivider />

        <SocialGoogleButton />
      </div>
    </AuthShell>
  )
}
