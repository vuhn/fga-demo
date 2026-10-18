import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import { api, session, type Tenant, type User } from "./api";

type Me = { user: User; tenants: Tenant[] };

type AuthState = {
  me: Me | null;
  loading: boolean;
  tenantId: string | null;
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
  refresh: () => Promise<void>;
  selectTenant: (id: string | null) => void;
};

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [me, setMe] = useState<Me | null>(null);
  const [loading, setLoading] = useState(true);
  const [tenantId, setTenantId] = useState<string | null>(session.tenantId);

  const selectTenant = useCallback((id: string | null) => {
    session.tenantId = id;
    setTenantId(id);
  }, []);

  const refresh = useCallback(async () => {
    if (!session.token) {
      setMe(null);
      setLoading(false);
      return;
    }
    try {
      const m = await api<Me>("GET", "/me");
      setMe(m);
      // Drop a selected tenant the user can no longer read.
      const current = session.tenantId;
      if (current && !m.tenants.some((t) => t.id === current)) selectTenant(m.tenants[0]?.id ?? null);
      if (!current && m.tenants.length > 0) selectTenant(m.tenants[0].id);
    } catch {
      session.token = null;
      setMe(null);
    } finally {
      setLoading(false);
    }
  }, [selectTenant]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const login = async (email: string, password: string) => {
    const res = await api<{ access_token: string }>("POST", "/auth/login", { email, password });
    session.token = res.access_token;
    selectTenant(null);
    await refresh();
  };

  const logout = () => {
    session.token = null;
    selectTenant(null);
    setMe(null);
  };

  return (
    <AuthContext.Provider value={{ me, loading, tenantId, login, logout, refresh, selectTenant }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth outside AuthProvider");
  return ctx;
}
