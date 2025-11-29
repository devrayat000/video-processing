import { Link, Outlet, NavLink } from "react-router";
import { buttonVariants } from "@/components/ui/button";
import { Fragment } from "react/jsx-runtime";
import { Toaster } from "./ui/sonner";

const navItems = [
  { path: "/", label: "Dashboard" },
  { path: "/upload", label: "Upload" },
  { path: "/videos", label: "Videos" },
];

export default function Layout() {
  return (
    <Fragment>
      <div className="min-h-screen">
        <nav className="border-b bg-background/95 backdrop-blur supports-backdrop-filter:bg-background/60">
          <div className="container mx-auto px-4 py-3 flex items-center gap-6">
            <Link to="/" className="font-semibold text-lg">
              Video Processing
            </Link>
            <div className="flex gap-2">
              {navItems.map((item) => (
                <NavLink
                  key={item.path}
                  className={(props) =>
                    buttonVariants({
                      variant: props.isActive ? "default" : "ghost",
                      size: "sm",
                    })
                  }
                  to={item.path}
                >
                  {item.label}
                </NavLink>
              ))}
            </div>
          </div>
        </nav>
        <div className="container mx-auto">
          <Outlet />
        </div>
      </div>
      <Toaster />
    </Fragment>
  );
}
