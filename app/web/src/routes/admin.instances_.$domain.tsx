// SPDX-License-Identifier: AGPL-3.0-only
import { Link } from "@my/ui";
import { createFileRoute, type ErrorComponentProps } from "@tanstack/react-router";
import { z } from "zod";
import { availabilityEventsQueryOptions, DomainName, InstanceDetail, instanceQueryOptions } from "../features/admin";
import { requireSignIn } from "../lib/auth/requireSignIn";
import { useT } from "../lib/i18n";
import { AdminError, loadWithPermission } from "./-admin/adminRoute";
import { PageHeading } from "./-layout/PageHeading";

/** The `_` keeps the page out of the list's layout at `/admin/instances`. */
const paramsSchema = z.object({ domain: z.string().min(1).max(253) });

/** One remote server's health and the history of its availability (federation.md §7.3), for manage_domains. */
export const Route = createFileRoute("/admin/instances_/$domain")({
  params: { parse: (params) => paramsSchema.parse(params) },
  beforeLoad: requireSignIn,
  loader: ({ context, params: { domain } }) =>
    loadWithPermission(context, "manage_domains", () =>
      Promise.all([
        context.queryClient.query({ ...instanceQueryOptions(context.api, domain), staleTime: "static" }),
        context.queryClient.infiniteQuery({ ...availabilityEventsQueryOptions(context.api, domain), staleTime: "static" }),
      ]),
    ),
  component: InstancePage,
  errorComponent: InstanceError,
});

function InstanceHeading() {
  const t = useT();
  const { domain } = Route.useParams();
  return (
    <div className="flex flex-col items-start gap-1">
      <PageHeading>
        <span className="wrap-anywhere">
          <DomainName domain={domain} />
        </span>
      </PageHeading>
      <Link href="/admin/instances" variant="standalone" className="text-sm">
        {t("client:admin.instance.back")}
      </Link>
    </div>
  );
}

function InstancePage() {
  const { domain } = Route.useParams();
  return (
    <>
      <InstanceHeading />
      {/* Another server is another page: nothing of the previous one carries over (web-client.md R10). */}
      <InstanceDetail key={domain} domain={domain} />
    </>
  );
}

function InstanceError(props: ErrorComponentProps) {
  const t = useT();
  return <AdminError {...props} permission="manage_domains" heading={t("client:admin.instances.title")} />;
}
