import { SignUp } from "@clerk/nextjs";

export default function SignUpPage() {
  return (
    <main className="flex min-h-screen items-center justify-center bg-gray-50">
      <SignUp forceRedirectUrl="/chat" />
    </main>
  );
}
