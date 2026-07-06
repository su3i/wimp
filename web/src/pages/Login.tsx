import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Eye, EyeOff, User, Lock, ArrowLeft } from "lucide-react";
import iconSrc from "@/assets/icon.svg";
import { authService } from "@/services/auth.service";
import { useAuthStore } from "@/store/auth";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";

type Phase = "credentials" | "mfa";

export function Login() {
  const navigate = useNavigate();
  const { setAuth } = useAuthStore();

  const [phase, setPhase] = useState<Phase>("credentials");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [mfaCode, setMfaCode] = useState("");
  const [challengeId, setChallengeId] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  async function handleCredentials() {
    setError("");
    setLoading(true);

    try {
      const { data } = await authService.login(username, password);

      if ("mfa_required" in data && data.mfa_required) {
        setChallengeId(data.challenge_id);
        setPhase("mfa");
      } else if ("access_token" in data) {
        setAuth(data.access_token, data.refresh_token);
        navigate("/", { replace: true });
      }
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        "Invalid username or password.";
      setError(msg);
    } finally {
      setLoading(false);
    }
  }

  async function handleMfa() {
    setError("");
    setLoading(true);

    try {
      const { data } = await authService.mfa(challengeId, mfaCode);
      setAuth(data.access_token, data.refresh_token);
      navigate("/", { replace: true });
    } catch {
      setError("Invalid authentication code. Please try again.");
      setMfaCode("");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className='min-h-screen bg-canvas flex items-center justify-center p-4'>
      <div className='w-full max-w-[450px]'>
        {/* Brand */}
        <div className='mb-8 flex justify-center'>
          <img src={iconSrc} alt='wimp' className='size-12 rounded-xl' />
        </div>

        {/* Card */}
        <div className='rounded-lg border border-rim bg-surface p-7'>
          {phase === "credentials" ? (
            <form
              onSubmit={(e) => {
                e.preventDefault();
                void handleCredentials();
              }}
              className='space-y-4'
            >
              <div>
                <p className='text-sm font-medium text-ink'>Sign In</p>
                <p className='mt-0.5 text-xs text-ink-faint'>Enter your credentials to continue</p>
              </div>

              <Input
                label='Username'
                type='text'
                placeholder='admin'
                autoComplete='username'
                leftIcon={<User className='size-3.5' />}
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required
              />

              <Input
                label='Password'
                type={showPassword ? "text" : "password"}
                placeholder='••••••••'
                autoComplete='current-password'
                leftIcon={<Lock className='size-3.5' />}
                rightSlot={
                  <button
                    type='button'
                    onClick={() => setShowPassword((v) => !v)}
                    className='cursor-pointer text-ink-faint hover:text-ink-dim transition-colors'
                    tabIndex={-1}
                  >
                    {showPassword ? <EyeOff className='size-3.5' /> : <Eye className='size-3.5' />}
                  </button>
                }
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
              />

              {error && (
                <p className='rounded-md border border-danger/20 bg-danger/5 px-3 py-2 text-xs text-danger'>
                  {error}
                </p>
              )}

              <Button type='submit' className='w-full' loading={loading}>
                Sign In
              </Button>
            </form>
          ) : (
            <form
              onSubmit={(e) => {
                e.preventDefault();
                void handleMfa();
              }}
              className='space-y-4'
            >
              <div>
                <p className='text-sm font-medium text-ink'>Two-factor authentication</p>
                <p className='mt-0.5 text-xs text-ink-faint'>
                  Enter the 6-digit code from your authenticator app
                </p>
              </div>

              <Input
                label='Authentication Code'
                type='text'
                inputMode='numeric'
                pattern='[0-9]{6}'
                maxLength={6}
                placeholder='000000'
                autoComplete='one-time-code'
                autoFocus
                value={mfaCode}
                onChange={(e) => setMfaCode(e.target.value.replace(/\D/g, ""))}
                required
                className='tracking-[0.4em] text-center font-mono text-base'
              />

              {error && (
                <p className='rounded-md border border-danger/20 bg-danger/5 px-3 py-2 text-xs text-danger'>
                  {error}
                </p>
              )}

              <Button type='submit' className='w-full' loading={loading} disabled={mfaCode.length !== 6}>
                Verify
              </Button>

              <button
                type='button'
                onClick={() => {
                  setPhase("credentials");
                  setError("");
                  setMfaCode("");
                }}
                className='flex items-center gap-1.5 text-xs text-ink-faint hover:text-ink-dim transition-colors mx-auto'
              >
                <ArrowLeft className='size-3' /> Back to sign in
              </button>
            </form>
          )}
        </div>
      </div>
    </div>
  );
}
