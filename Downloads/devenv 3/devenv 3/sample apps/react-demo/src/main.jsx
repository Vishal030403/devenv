import React from "react";
import { createRoot } from "react-dom/client";

const steps = [
  { icon: "⚙️", title: "devenv setup", desc: "Spins up a Kind cluster, deploys Jenkins & a local Docker registry, and scaffolds your project — all in one command." },
  { icon: "🚀", title: "devenv run",   desc: "Runs the full CI/CD pipeline: lint → unit tests → Docker build → K8s manifest validation → deploy → health check." },
  { icon: "📊", title: "devenv status", desc: "Shows the live state of your cluster, pods, registry, and Jenkins at a glance." },
  { icon: "🧹", title: "devenv down",  desc: "Tears everything down cleanly — cluster, Jenkins, registry, and local images removed in one shot." },
];

const features = [
  { color: "#6366f1", label: "Auto Detection",   detail: "Detects React, Express, Django, FastAPI, Spring Boot automatically." },
  { color: "#0ea5e9", label: "Kind Cluster",      detail: "Creates an isolated Kubernetes cluster locally using Kind + NGINX ingress." },
  { color: "#10b981", label: "Jenkins Built-in",  detail: "Jenkins deployed via Helm inside the cluster. No manual setup needed." },
  { color: "#f59e0b", label: "Local Registry",    detail: "Docker registry on localhost:5000 — push and pull without Docker Hub." },
  { color: "#ef4444", label: "Auto Rollback",     detail: "Failed health checks trigger automatic kubectl rollout undo instantly." },
  { color: "#8b5cf6", label: "Zero Config",       detail: "No YAML to write. Drop your project in and devenv handles the rest." },
];

const styles = {
  page: {
    margin: 0,
    fontFamily: "'Segoe UI', system-ui, -apple-system, sans-serif",
    background: "linear-gradient(135deg, #0f0c29, #302b63, #24243e)",
    minHeight: "100vh",
    color: "#fff",
  },
  hero: {
    textAlign: "center",
    padding: "80px 24px 60px",
  },
  badge: {
    display: "inline-block",
    background: "rgba(99,102,241,0.25)",
    border: "1px solid rgba(99,102,241,0.5)",
    borderRadius: 999,
    padding: "6px 18px",
    fontSize: 13,
    letterSpacing: 1,
    marginBottom: 28,
    color: "#a5b4fc",
  },
  h1: {
    fontSize: "clamp(2.4rem, 6vw, 4rem)",
    fontWeight: 800,
    margin: "0 0 20px",
    background: "linear-gradient(90deg, #818cf8, #38bdf8, #34d399)",
    WebkitBackgroundClip: "text",
    WebkitTextFillColor: "transparent",
    lineHeight: 1.1,
  },
  subtitle: {
    fontSize: "clamp(1rem, 2vw, 1.25rem)",
    color: "#94a3b8",
    maxWidth: 560,
    margin: "0 auto 40px",
    lineHeight: 1.7,
  },
  cmdBox: {
    display: "inline-flex",
    alignItems: "center",
    gap: 12,
    background: "rgba(255,255,255,0.05)",
    border: "1px solid rgba(255,255,255,0.12)",
    borderRadius: 12,
    padding: "14px 28px",
    fontSize: 15,
    fontFamily: "monospace",
    color: "#7dd3fc",
    backdropFilter: "blur(8px)",
  },
  section: {
    maxWidth: 1100,
    margin: "0 auto",
    padding: "0 24px 80px",
  },
  sectionTitle: {
    textAlign: "center",
    fontSize: "1.6rem",
    fontWeight: 700,
    marginBottom: 40,
    color: "#e2e8f0",
  },
  stepsGrid: {
    display: "grid",
    gridTemplateColumns: "repeat(auto-fit, minmax(240px, 1fr))",
    gap: 20,
  },
  stepCard: {
    background: "rgba(255,255,255,0.04)",
    border: "1px solid rgba(255,255,255,0.09)",
    borderRadius: 16,
    padding: "28px 24px",
    transition: "transform 0.2s",
  },
  stepIcon: { fontSize: 32, marginBottom: 12 },
  stepTitle: {
    fontFamily: "monospace",
    fontSize: 16,
    fontWeight: 700,
    color: "#818cf8",
    marginBottom: 8,
  },
  stepDesc: { fontSize: 14, color: "#94a3b8", lineHeight: 1.6 },
  featGrid: {
    display: "grid",
    gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))",
    gap: 16,
  },
  footer: {
    textAlign: "center",
    padding: "32px 24px",
    borderTop: "1px solid rgba(255,255,255,0.08)",
    color: "#475569",
    fontSize: 13,
  },
};

function FeatureCard({ color, label, detail }) {
  return (
    <div style={{
      background: "rgba(255,255,255,0.04)",
      border: `1px solid ${color}44`,
      borderRadius: 14,
      padding: "22px 20px",
    }}>
      <div style={{
        width: 10, height: 10, borderRadius: "50%",
        background: color, marginBottom: 12,
        boxShadow: `0 0 10px ${color}`,
      }} />
      <div style={{ fontWeight: 700, fontSize: 15, marginBottom: 6, color: "#e2e8f0" }}>{label}</div>
      <div style={{ fontSize: 13, color: "#64748b", lineHeight: 1.6 }}>{detail}</div>
    </div>
  );
}

function App() {
  return (
    <div style={styles.page}>
      <div style={styles.hero}>
        <div style={styles.badge}>⚡ LOCAL CI/CD PLATFORM</div>
        <h1 style={styles.h1}>local cicd  </h1>
        <p style={styles.subtitle}>
          One CLI that sets up Kubernetes, Jenkins, and a Docker registry on your machine —
          then builds, deploys, and health-checks your app automatically.
        </p>
        <div style={styles.cmdBox}>
          <span style={{ color: "#475569" }}>$</span>
          <span>devenv setup &amp;&amp; devenv run</span>
        </div>
      </div>

      <div style={styles.section}>
        <h2 style={styles.sectionTitle}>Four commands. Full pipeline.</h2>
        <div style={styles.stepsGrid}>
          {steps.map((s) => (
            <div key={s.title} style={styles.stepCard}>
              <div style={styles.stepIcon}>{s.icon}</div>
              <div style={styles.stepTitle}>{s.title}</div>
              <div style={styles.stepDesc}>{s.desc}</div>
            </div>
          ))}
        </div>
      </div>

      <div style={styles.section}>
        <h2 style={styles.sectionTitle}>Everything included</h2>
        <div style={styles.featGrid}>
          {features.map((f) => (
            <FeatureCard key={f.label} {...f} />
          ))}
        </div>
      </div>

      <footer style={styles.footer}>
        Built by Team Alpha · devenv Week 1 · Deployed via devenv run
      </footer>
    </div>
  );
}

createRoot(document.getElementById("root")).render(<App />);
