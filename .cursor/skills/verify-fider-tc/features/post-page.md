# Post page: votes and comments

`/posts/<number>/<slug>` shows a post's title, body, tags, vote control, voters and discussion. Signed-in users vote and comment. Staff get the actions menu (Respond, Edit, and so on).

## Sub-features

- `post-view` shows the title (h1), body, author and created and last-activity times.
- `post-vote` toggles the vote with **Upvote** and **Downvote**, updating the count and the Voters list.
- `post-comment` posts a comment from the **Leave a comment** composer with **Submit comment**, and the comment appears in the **Discussion** region.
- `post-comment-sort` sorts the discussion with the `Sort discussion` combobox (Most liked, Most disliked, Most replies, Latest).
- `post-follow` toggles the **Following** button.
- `post-tags` uses **Edit tags** (staff) to assign a tag, which then appears on the post.

## How to get to it (user POV)

- Click a post title in the home list.
- Open `/posts/<number>` directly. It redirects to the slugged URL.

## Driving it with Playwright (control-fider)

Preconditions:

- A post exists, created via the composer in [post-list](./post-list.md). The author has auto-voted, so the count is `1`.

- **View.** Go to the post. `getByRole("heading", { level: 1 })` equals the title, and the vote count text is `1`.
- **Vote toggle.** Click `getByRole("button", { name: "Upvote" })`. The count and Voters list change. Check with `select count(*) from post_votes where post_id=(select id from posts where number=<n>)`.
- **Comment.** Type into the comment editor under "Leave a comment". `getByRole("button", { name: "Submit comment" })` becomes enabled. Click it. The comment text appears inside `getByRole("region", { name: "Discussion" })`. Check with `select content from comments`.
- **Visitor view.** In an anonymous context, the comment and vote count are visible, and voting opens the sign-in modal.
- **Proof.** Save `post-page/after.aria.txt`, `post-page/after.png`, and the DB rows.

## Gotchas

- `Submit comment` is disabled until the editor has content. The editor is a rich-text area, so use `click()` then `keyboard.type()`, because `fill()` may not trigger it.
- The author's own vote is created automatically when the post is created. Account for it in the before count.
- Mentions (`@`) open a selector popup that can swallow Enter.
