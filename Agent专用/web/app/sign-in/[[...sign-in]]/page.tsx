import { SignIn } from "@clerk/nextjs";

export default function SignInPage() {
  return (
    <main className="flex min-h-screen items-center justify-center bg-gray-50">
      <SignIn forceRedirectUrl="/chat" />
    </main>
  );
}
