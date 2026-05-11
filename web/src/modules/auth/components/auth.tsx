"use client";

import * as React from "react";
import { useState, useId, useEffect } from "react";
import Link from "next/link";
import { Label as LabelPrimitive, Slot } from "radix-ui";
import { cva, type VariantProps } from "class-variance-authority";
import { Eye, EyeOff } from "lucide-react";
import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";
import { Button } from "@/components/ui/button";

function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export interface TypewriterProps {
  text: string | string[];
  speed?: number;
  cursor?: string;
  loop?: boolean;
  deleteSpeed?: number;
  delay?: number;
  className?: string;
}

export function Typewriter({
  text,
  speed = 100,
  cursor = "|",
  loop = false,
  deleteSpeed = 50,
  delay = 1500,
  className,
}: TypewriterProps) {
  const [displayText, setDisplayText] = useState("");
  const [currentIndex, setCurrentIndex] = useState(0);
  const [isDeleting, setIsDeleting] = useState(false);
  const [textArrayIndex, setTextArrayIndex] = useState(0);

  const textArray = Array.isArray(text) ? text : [text];
  const currentText = textArray[textArrayIndex] || "";

  useEffect(() => {
    if (!currentText) return;

    const timeout = setTimeout(
      () => {
        if (!isDeleting) {
          if (currentIndex < currentText.length) {
            setDisplayText((prev) => prev + currentText[currentIndex]);
            setCurrentIndex((prev) => prev + 1);
          } else if (loop) {
            setTimeout(() => setIsDeleting(true), delay);
          }
        } else {
          if (displayText.length > 0) {
            setDisplayText((prev) => prev.slice(0, -1));
          } else {
            setIsDeleting(false);
            setCurrentIndex(0);
            setTextArrayIndex((prev) => (prev + 1) % textArray.length);
          }
        }
      },
      isDeleting ? deleteSpeed : speed,
    );

    return () => clearTimeout(timeout);
  }, [
    currentIndex,
    isDeleting,
    currentText,
    loop,
    speed,
    deleteSpeed,
    delay,
    displayText,
    text,
  ]);

  return (
    <span className={className}>
      {displayText}
      <span className="animate-pulse">{cursor}</span>
    </span>
  );
}

const labelVariants = cva(
  "text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70",
);

const Label = React.forwardRef<
  React.ElementRef<typeof LabelPrimitive.Root>,
  React.ComponentPropsWithoutRef<typeof LabelPrimitive.Root> &
    VariantProps<typeof labelVariants>
>(({ className, ...props }, ref) => (
  <LabelPrimitive.Root
    ref={ref}
    className={cn(labelVariants(), className)}
    {...props}
  />
));
Label.displayName = LabelPrimitive.Root.displayName;

// const buttonVariants = cva(
//   "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0",
//   {
//     variants: {
//       variant: {
//         default: "bg-primary text-primary-foreground hover:bg-primary/90",
//         destructive:
//           "bg-destructive text-destructive-foreground hover:bg-destructive/90",
//         outline:
//           "border border-input dark:border-input/50 bg-background hover:bg-accent hover:text-accent-foreground",
//         secondary:
//           "bg-secondary text-secondary-foreground hover:bg-secondary/80",
//         ghost: "hover:bg-accent hover:text-accent-foreground",
//         link: "text-primary-foreground/60 underline-offset-4 hover:underline",
//       },
//       size: {
//         default: "h-10 px-4 py-2",
//         sm: "h-9 rounded-md px-3",
//         lg: "h-12 rounded-md px-6",
//         icon: "h-8 w-8",
//       },
//     },
//     defaultVariants: {
//       variant: "default",
//       size: "default",
//     },
//   },
// );
// interface ButtonProps
//   extends
//     React.ButtonHTMLAttributes<HTMLButtonElement>,
//     VariantProps<typeof buttonVariants> {
//   asChild?: boolean;
// }
// const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
//   ({ className, variant, size, asChild = false, ...props }, ref) => {
//     const Comp = asChild ? Slot.Root : "button";
//     return (
//       <Comp
//         className={cn(buttonVariants({ variant, size, className }))}
//         ref={ref}
//         {...props}
//       />
//     );
//   },
// );
// Button.displayName = "Button";

// const Input = React.forwardRef<HTMLInputElement, React.ComponentProps<"input">>(
//   ({ className, type, ...props }, ref) => {
//     return (
//       <input
//         type={type}
//         className={cn(
//           "flex h-10 w-full rounded-lg border border-input dark:border-input/50 bg-background px-3 py-3 text-sm text-foreground shadow-sm shadow-black/5 transition-shadow placeholder:text-muted-foreground/70 focus-visible:bg-accent focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50",
//           className,
//         )}
//         ref={ref}
//         {...props}
//       />
//     );
//   },
// );
// Input.displayName = "Input";

// export interface PasswordInputProps extends React.InputHTMLAttributes<HTMLInputElement> {
//   label?: string;
// }
// const PasswordInput = React.forwardRef<HTMLInputElement, PasswordInputProps>(
//   ({ className, label, ...props }, ref) => {
//     const id = useId();
//     const [showPassword, setShowPassword] = useState(false);
//     const togglePasswordVisibility = () => setShowPassword((prev) => !prev);
//     return (
//       <div className="grid w-full items-center gap-2">
//         {label && <Label htmlFor={id}>{label}</Label>}
//         <div className="relative">
//           <Input
//             id={id}
//             type={showPassword ? "text" : "password"}
//             className={cn("pe-10", className)}
//             ref={ref}
//             {...props}
//           />
//           <button
//             type="button"
//             onClick={togglePasswordVisibility}
//             className="absolute inset-y-0 end-0 flex h-full w-10 items-center justify-center text-muted-foreground/80 transition-colors hover:text-foreground focus-visible:text-foreground focus-visible:outline-none disabled:pointer-events-none disabled:opacity-50"
//             aria-label={showPassword ? "Hide password" : "Show password"}
//           >
//             {showPassword ? (
//               <EyeOff className="size-4" aria-hidden="true" />
//             ) : (
//               <Eye className="size-4" aria-hidden="true" />
//             )}
//           </button>
//         </div>
//       </div>
//     );
//   },
// );
// PasswordInput.displayName = "PasswordInput";

export type AuthHeroContent = {
  image: {
    src: string;
    alt: string;
  };
  quote: {
    text: string;
  };
};

export const defaultLoginContent: AuthHeroContent = {
  image: {
    src: "https://i.ibb.co/XrkdGrrv/original-ccdd6d6195fff2386a31b684b7abdd2e-removebg-preview.png",
    alt: "Astronaut illustration for sign in",
  },
  quote: {
    text: "Welcome Back! The journey continues.",
  },
};

export const defaultRegisterContent: AuthHeroContent = {
  image: {
    src: "https://i.ibb.co/HTZ6DPsS/original-33b8479c324a5448d6145b3cad7c51e7-removebg-preview.png",
    alt: "Astronaut illustration for sign up",
  },
  quote: {
    text: "Create an account. A new chapter awaits.",
  },
};

export function GoogleIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 256 262"
      className="h-4 w-4 shrink-0 sm:h-5 sm:w-5"
      aria-hidden="true"
    >
      <path
        fill="#4285f4"
        d="M255.878 133.451c0-10.734-.871-18.567-2.756-26.69H130.55v48.448h71.947c-1.45 12.04-9.283 30.172-26.69 42.356l-.244 1.622l38.755 30.023l2.685.268c24.659-22.774 38.875-56.282 38.875-96.027"
      />
      <path
        fill="#34a853"
        d="M130.55 261.1c35.248 0 64.839-11.605 86.453-31.622l-41.196-31.913c-11.024 7.688-25.82 13.055-45.257 13.055c-34.523 0-63.824-22.773-74.269-54.25l-1.531.13l-40.298 31.187l-.527 1.465C35.393 231.798 79.49 261.1 130.55 261.1"
      />
      <path
        fill="#fbbc05"
        d="M56.281 156.37c-2.756-8.123-4.351-16.827-4.351-25.82c0-8.994 1.595-17.697 4.206-25.82l-.073-1.73L15.26 71.312l-1.335.635C5.077 89.644 0 109.517 0 130.55s5.077 40.905 13.925 58.602z"
      />
      <path
        fill="#eb4335"
        d="M130.55 50.479c24.514 0 41.05 10.589 50.479 19.438l36.844-35.974C195.245 12.91 165.798 0 130.55 0C79.49 0 35.393 29.301 13.925 71.947l42.211 32.783c10.59-31.477 39.891-54.251 74.414-54.251"
      />
    </svg>
  );
}

export function SocialGoogleButton() {
  return (
    <Button
      type="button"
      variant="outline"
      onClick={() => console.log("UI: Google button clicked")}
      className="h-11 w-full border-border bg-white text-sm font-medium text-foreground shadow-sm hover:bg-accent/40 sm:h-12"
    >
      <GoogleIcon />
      <span>Continue with Google</span>
    </Button>
  );
}

export function AuthDivider() {
  return (
    <div className="flex items-center gap-3 px-1">
      <span className="h-px flex-1 bg-border" />
      <span className="whitespace-nowrap text-sm text-muted-foreground">
        Or continue with
      </span>
      <span className="h-px flex-1 bg-border" />
    </div>
  );
}

export function AuthHero({
  image,
  quote,
}: {
  image: AuthHeroContent["image"];
  quote: AuthHeroContent["quote"];
}) {
  return (
    <div className="relative hidden min-h-screen overflow-hidden bg-white lg:block">
      <div className="absolute inset-0 bg-gradient-to-b from-white/10 via-transparent to-white/5" />
      <div className="absolute inset-x-0 bottom-0 h-32 bg-gradient-to-t from-white to-transparent" />

      <div className="relative flex min-h-screen flex-col items-center justify-center px-8 py-10">
        <img
          src={image.src}
          alt={image.alt}
          className="h-auto w-[72%] max-w-[520px] object-contain"
        />

        <blockquote className="mt-4 max-w-lg space-y-2 text-center">
          <p className="text-base font-medium leading-tight text-foreground sm:text-lg">
            “<Typewriter key={quote.text} text={quote.text} speed={55} />”
          </p>
        </blockquote>
      </div>
    </div>
  );
}

export function AuthShell({
  title,
  subtitle,
  hero,
  onSubmit,
  children,
}: {
  title: string;
  subtitle: string;
  hero: AuthHeroContent;
  onSubmit: React.FormEventHandler<HTMLFormElement>;
  children: React.ReactNode;
}) {
  return (
    <section className="min-h-screen bg-background">
      <style>{`
        input[type="password"]::-ms-reveal,
        input[type="password"]::-ms-clear {
          display: none;
        }
      `}</style>
      <div className="grid min-h-screen lg:grid-cols-2">
        <div className="flex min-h-screen items-center justify-center px-4 py-8 sm:px-6 lg:px-10">
          <div className="w-full max-w-[400px]">
            <form onSubmit={onSubmit} autoComplete="on" className="space-y-6">
              <div className="space-y-3 text-center lg:text-left">
                <h1 className="text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
                  {title}
                </h1>
                <p className="text-sm text-muted-foreground">{subtitle}</p>
              </div>

              {children}
            </form>
          </div>
        </div>

        <AuthHero image={hero.image} quote={hero.quote} />
      </div>
    </section>
  );
}

// export function AuthUI() {
//   const [isSignIn, setIsSignIn] = useState(true);

//   return (
//     <div className="min-h-screen">
//       {isSignIn ? <LoginForm /> : <SignUpForm />}
//       <button
//         type="button"
//         onClick={() => setIsSignIn((prev) => !prev)}
//         className="fixed bottom-4 right-4 rounded-full bg-foreground px-4 py-2 text-sm font-medium text-background shadow-lg"
//       >
//         Switch to {isSignIn ? "Sign up" : "Sign in"}
//       </button>
//     </div>
//   );
// }
