import { useEffect, useState, type FormEvent } from "react";

interface PasswordDialogProps {
  description: string;
  open: boolean;
  requireCurrentPassword?: boolean;
  submitLabel: string;
  title: string;
  onClose: () => void;
  onSubmit: (currentPassword: string, newPassword: string) => Promise<void>;
}

export function PasswordDialog({
  description,
  open,
  requireCurrentPassword = false,
  submitLabel,
  title,
  onClose,
  onSubmit,
}: PasswordDialogProps) {
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [passwordVisible, setPasswordVisible] = useState(false);
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!open) return;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setCurrentPassword("");
    setNewPassword("");
    setConfirmation("");
    setPasswordVisible(false);
    setError("");
    setSubmitting(false);
  }, [open]);

  if (!open) return null;

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    if (newPassword.length < 8) {
      setError("Password baru minimal 8 karakter.");
      return;
    }
    if (newPassword !== confirmation) {
      setError("Konfirmasi password tidak sama.");
      return;
    }

    setSubmitting(true);
    try {
      await onSubmit(currentPassword, newPassword);
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Unable to change password.");
      setSubmitting(false);
    }
  }

  return (
    <div
      aria-modal="true"
      className="fixed inset-0 z-50 grid place-items-center bg-black/70 px-4 backdrop-blur-sm"
      role="dialog"
    >
      <div className="w-full max-w-md rounded-2xl border border-white/10 bg-[#101815] p-6 shadow-2xl">
        <h2 className="font-display text-2xl text-stone-100">{title}</h2>
        <p className="mt-2 text-sm leading-6 text-stone-400">{description}</p>

        <form className="mt-5 grid gap-3" onSubmit={submit}>
          {requireCurrentPassword ? (
            <input
              aria-label="Current password"
              autoComplete="current-password"
              className="admin-input"
              onChange={(event) => setCurrentPassword(event.target.value)}
              placeholder="Current password"
              required
              type={passwordVisible ? "text" : "password"}
              value={currentPassword}
            />
          ) : null}
          <input
            aria-label="New password"
            autoComplete="new-password"
            className="admin-input"
            minLength={8}
            onChange={(event) => setNewPassword(event.target.value)}
            placeholder="New password"
            required
            type={passwordVisible ? "text" : "password"}
            value={newPassword}
          />
          <input
            aria-label="Confirm new password"
            autoComplete="new-password"
            className="admin-input"
            minLength={8}
            onChange={(event) => setConfirmation(event.target.value)}
            placeholder="Confirm new password"
            required
            type={passwordVisible ? "text" : "password"}
            value={confirmation}
          />
          <label className="flex items-center gap-2 text-xs text-stone-400">
            <input
              checked={passwordVisible}
              onChange={(event) => setPasswordVisible(event.target.checked)}
              type="checkbox"
            />
            Show password
          </label>

          {error ? (
            <div className="rounded-xl border border-red-400/20 bg-red-950/30 px-3 py-2 text-sm text-red-200">
              {error}
            </div>
          ) : null}

          <div className="mt-2 flex justify-end gap-2">
            <button className="admin-button" disabled={submitting} onClick={onClose} type="button">
              Cancel
            </button>
            <button className="admin-primary" disabled={submitting} type="submit">
              {submitting ? "Saving..." : submitLabel}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
