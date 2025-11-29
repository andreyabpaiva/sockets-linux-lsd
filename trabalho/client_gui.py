import base64
import binascii
import json
import socket
import threading
import tkinter as tk
from tkinter import filedialog, messagebox, scrolledtext


class CompileClientApp:
    def __init__(self) -> None:
        self.root = tk.Tk()
        self.root.title("Remote Compile Client")
        self.root.geometry("900x700")

        self.host_var = tk.StringVar(value="127.0.0.1")
        self.port_var = tk.IntVar(value=6000)
        self.status_var = tk.StringVar(value="Idle")
        self.last_binary: bytes | None = None

        self._build_layout()

    def _build_layout(self) -> None:
        connection_frame = tk.Frame(self.root)
        connection_frame.pack(fill=tk.X, padx=8, pady=4)

        tk.Label(connection_frame, text="Host:").pack(side=tk.LEFT)
        tk.Entry(connection_frame, textvariable=self.host_var, width=18).pack(side=tk.LEFT, padx=4)
        tk.Label(connection_frame, text="Port:").pack(side=tk.LEFT)
        tk.Entry(connection_frame, textvariable=self.port_var, width=6).pack(side=tk.LEFT, padx=4)
        tk.Button(connection_frame, text="Run Code", command=self.run_remote).pack(side=tk.LEFT, padx=8)
        tk.Button(connection_frame, text="Download Binary", command=self.download_binary).pack(side=tk.LEFT)
        tk.Label(connection_frame, textvariable=self.status_var).pack(side=tk.RIGHT)

        editor_label = tk.Label(self.root, text="Source Code (C)")
        editor_label.pack(anchor=tk.W, padx=8)

        self.code_text = scrolledtext.ScrolledText(self.root, height=15, wrap=tk.WORD)
        self.code_text.pack(fill=tk.BOTH, expand=True, padx=8, pady=4)

        tk.Label(self.root, text="Compile Diagnostics").pack(anchor=tk.W, padx=8)
        self.error_text = scrolledtext.ScrolledText(self.root, height=8, bg="#1e1e1e", fg="#f0f0f0")
        self.error_text.pack(fill=tk.BOTH, expand=True, padx=8, pady=4)

        tk.Label(self.root, text="Program Output").pack(anchor=tk.W, padx=8)
        self.output_text = scrolledtext.ScrolledText(self.root, height=8, bg="#101820", fg="#00ff66")
        self.output_text.pack(fill=tk.BOTH, expand=True, padx=8, pady=4)

        sample_program = """#include <stdio.h>

int main(void) {
    printf("Hello from remote compiler!\\n");
    return 0;
}
"""
        self.code_text.insert(tk.END, sample_program)

    def run_remote(self) -> None:
        code = self.code_text.get("1.0", tk.END).strip()
        if not code:
            messagebox.showwarning("Input required", "Please enter source code first.")
            return

        self.status_var.set("Sending request...")
        self.error_text.delete("1.0", tk.END)
        self.output_text.delete("1.0", tk.END)
        self.last_binary = None

        thread = threading.Thread(target=self._send_request, args=(code,), daemon=True)
        thread.start()

    def _send_request(self, code: str) -> None:
        host = self.host_var.get().strip()
        port = self.port_var.get()
        payload = json.dumps({"language": "c", "source": code, "args": []}).encode("utf-8") + b"\n"

        try:
            with socket.create_connection((host, port), timeout=5) as sock:
                sock.sendall(payload)
                response_line = sock.makefile("r", encoding="utf-8").readline()
        except OSError as exc:
            self._update_status(f"Connection error: {exc}")
            return

        if not response_line:
            self._update_status("Empty response from server")
            return

        try:
            response = json.loads(response_line)
        except json.JSONDecodeError as exc:
            self._update_status(f"Invalid JSON response: {exc}")
            return

        self.root.after(0, self._apply_response, response)

    def _apply_response(self, response: dict) -> None:
        self.error_text.insert(tk.END, response.get("compile_stdout", ""))
        stderr = response.get("compile_stderr", "")
        if stderr:
            self.error_text.insert(tk.END, "\n" + stderr)

        run_stdout = response.get("run_stdout", "")
        run_stderr = response.get("run_stderr", "")
        if run_stdout:
            self.output_text.insert(tk.END, run_stdout)
        if run_stderr:
            self.output_text.insert(tk.END, "\n" + run_stderr)

        error_message = response.get("error")
        if error_message:
            self.status_var.set(f"Completed with error: {error_message}")
        else:
            self.status_var.set("Completed successfully")

        binary_b64 = response.get("binary_base64") or ""
        if binary_b64:
            try:
                self.last_binary = base64.b64decode(binary_b64)
            except (ValueError, binascii.Error):
                self.last_binary = None

    def download_binary(self) -> None:
        if not self.last_binary:
            messagebox.showinfo("No binary", "Run a successful compilation first.")
            return

        file_path = filedialog.asksaveasfilename(
            title="Save Executable",
            defaultextension=".out",
            filetypes=[("Executable", "*.out"), ("All Files", "*.*")],
        )
        if not file_path:
            return

        try:
            with open(file_path, "wb") as handle:
                handle.write(self.last_binary)
            messagebox.showinfo("Saved", f"Binary saved to {file_path}")
        except OSError as exc:
            messagebox.showerror("Save failed", f"Could not save file: {exc}")

    def _update_status(self, message: str) -> None:
        self.root.after(0, lambda: self.status_var.set(message))

    def run(self) -> None:
        self.root.mainloop()


if __name__ == "__main__":
    CompileClientApp().run()

