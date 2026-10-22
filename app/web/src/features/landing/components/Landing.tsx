// SPDX-License-Identifier: AGPL-3.0-only
import { Button, Cluster, EmptyState, Feed, Heading, Hint, Link, PostSkeleton, RegionError, Stack, Text, VisuallyHidden } from "@my/ui";
import { useId } from "react";
import { useT } from "../../../lib/i18n";
import { aboutPath } from "../../../lib/paths";
import { ServerThumbnail, SourceCodeLink } from "../../about";
import { DocumentsSection } from "../../documents";
import { useSignIn } from "../../session";
import { StatusCard } from "../../status";
import type { RecentPost } from "../api/landing";
import { type Landing as LandingData, type RecentPosts, type RegistrationMode, useLanding } from "../hooks/useLanding";

/**
 * The body of the landing page for people who are not signed in (`/`), under the page's heading, in the order of the
 * information page (`/about`): what the server is (its image and summary, how to sign in or create an account, its
 * description, and a few recent posts), what users agree to (its rules, then its terms of service and privacy
 * policy), and the links to the information page, which also tells how the server treats other servers and
 * crawlers and who runs it, and to its source code (AGPL-3.0 section 13).
 */
export function Landing() {
  const t = useT();
  const landing = useLanding();
  return (
    <Stack gap="8">
      <Stack gap="4">
        {landing.thumbnail !== undefined && <ServerThumbnail thumbnail={landing.thumbnail} />}
        {landing.summary !== "" && (
          // The administrator's text, in a language the app does not know: lang="" rather than the app's (web-client.md Y10).
          <Text as="p" dir="auto" lang="" size="lg">
            {landing.summary}
          </Text>
        )}
        <Entry mode={landing.registrationMode} signUpHref={landing.signUpHref} />
      </Stack>
      {landing.description.length > 0 && <About paragraphs={landing.description} />}
      {/* A sample of what the server is: its latest public posts. */}
      {landing.recentPosts.status !== "off" && <RecentPostsSection posts={landing.recentPosts} />}
      {/* What users agree to stands together: the rules, then the terms and policies. */}
      {landing.rules.length > 0 && <Rules rules={landing.rules} />}
      <DocumentsSection />
      {/* How the server treats other servers and crawlers, and how to reach the operators, are on the information
          page; what the server runs comes last. */}
      <p>
        <Link href={aboutPath} variant="standalone">
          {t("client:shell.about.more")}
        </Link>
      </p>
      {landing.sourceUrl !== undefined && (
        <p>
          <SourceCodeLink url={landing.sourceUrl} />
        </p>
      )}
    </Stack>
  );
}

/** The sign-up link's text for each mode in which people may sign up. */
const signUpLabels = {
  open: "client:landing.sign_up.open",
  approval: "client:landing.sign_up.approval",
  invite: "client:landing.sign_up.invite",
} as const;

/** What the mode means for someone about to sign up; an open server needs no note. */
const registrationNotes = {
  open: undefined,
  approval: "client:landing.registrations.approval",
  invite: "client:landing.registrations.invite",
} as const;

/** The ways in: signing in, and creating an account when the server takes new ones. */
function Entry({ mode, signUpHref }: { mode: RegistrationMode | undefined; signUpHref: string }) {
  const t = useT();
  const noteId = useId();
  // After signing in, the root address opens the home timeline.
  const { signIn, isPending } = useSignIn("/");
  const note = mode === undefined || mode === "closed" ? undefined : registrationNotes[mode];
  return (
    <Stack align="start" gap="3">
      <Cluster align="center" gap="6" rowGap="3">
        <Button variant="primary" onPress={signIn} isPending={isPending}>
          {t("client:landing.sign_in")}
        </Button>
        {mode !== undefined && mode !== "closed" && (
          // The registration page is the server's own; the link loads it in place of the app (lib/paths.ts).
          <Link href={signUpHref} variant="standalone" {...(note !== undefined && { "aria-describedby": noteId })}>
            {t(signUpLabels[mode])}
          </Link>
        )}
      </Cluster>
      {note !== undefined && <Hint id={noteId}>{t(note)}</Hint>}
      {mode === "closed" && <Hint>{t("client:landing.registrations.closed")}</Hint>}
    </Stack>
  );
}

function About({ paragraphs }: { paragraphs: readonly string[] }) {
  const t = useT();
  const headingId = useId();
  return (
    <Stack as="section" aria-labelledby={headingId} gap="3">
      <Heading level={2} id={headingId}>
        {t("client:landing.about.title")}
      </Heading>
      {paragraphs.map((paragraph) => (
        // Paragraphs of one text have no IDs; the text itself is the key (two equal paragraphs are not expected).
        <Text as="p" key={paragraph} dir="auto" lang="" whitespace="preLine">
          {paragraph}
        </Text>
      ))}
    </Stack>
  );
}

function Rules({ rules }: { rules: LandingData["rules"] }) {
  const t = useT();
  const headingId = useId();
  return (
    <Stack as="section" aria-labelledby={headingId} gap="3">
      <Heading level={2} id={headingId}>
        {t("client:landing.rules.title")}
      </Heading>
      <Stack as="ol" marker="decimal" gap="2">
        {rules.map((rule) => (
          // The item follows the page's direction, so that its number stays in the list's padding; the rule's text
          // follows its own (dir="auto"); lang="" because the administrator's text has no known language.
          <li key={rule.id} lang="">
            <Text as="p" dir="auto" weight="medium">
              {rule.text}
            </Text>
            {rule.hint !== undefined && <Hint dir="auto">{rule.hint}</Hint>}
          </li>
        ))}
      </Stack>
    </Stack>
  );
}

/** Placeholder cards while the recent posts load. */
const placeholders = ["first", "second"] as const;

function RecentPostsSection({ posts }: { posts: Exclude<RecentPosts, { status: "off" }> }) {
  const t = useT();
  const headingId = useId();
  return (
    <Stack as="section" aria-labelledby={headingId} gap="3">
      <Heading level={2} id={headingId}>
        {t("client:landing.posts.title")}
      </Heading>
      <RecentPostsBody posts={posts} headingId={headingId} />
      <p>
        <Link href="/local" variant="standalone">
          {t("client:landing.posts.more")}
        </Link>
      </p>
    </Stack>
  );
}

function RecentPostsBody({ posts, headingId }: { posts: Exclude<RecentPosts, { status: "off" }>; headingId: string }) {
  const t = useT();
  switch (posts.status) {
    case "loading":
      return (
        <Stack aria-busy="true">
          <VisuallyHidden>{t("client:timeline.loading")}</VisuallyHidden>
          {placeholders.map((key) => (
            <PostSkeleton key={key} />
          ))}
        </Stack>
      );
    case "error":
      return (
        <RegionError
          heading={t("client:landing.posts.failed")}
          headingLevel={3}
          reason={posts.reason}
          retryLabel={t("client:timeline.retry")}
          onRetry={posts.retry}
          isRetrying={posts.isRetrying}
        />
      );
    case "ready":
      if (posts.posts.length === 0) return <EmptyState headingLevel={3} heading={t("client:landing.posts.empty")} />;
      return (
        <Stack>
          {/* The feed role has no keyboard convention, so its keys are told before it (Feed's documentation). */}
          <VisuallyHidden>
            <p>{t("client:timeline.keyboard_hint")}</p>
          </VisuallyHidden>
          <Feed<RecentPost>
            aria-labelledby={headingId}
            items={posts.posts}
            getKey={(post) => post.id}
            // A sample to read: acting on a post takes signing in first.
            renderItem={(post) => <StatusCard status={post} withActions={false} />}
            isLoading={false}
            hasMore={false}
            onLoadMore={() => undefined}
            loadMoreLabel={t("client:timeline.load_more")}
            loadingLabel={t("client:timeline.loading")}
          />
        </Stack>
      );
  }
}
