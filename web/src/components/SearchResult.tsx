import { decode } from 'he'
import marked from 'marked'
import FileIcon from 'mdi-react/FileIcon'
import React from 'react'
import { ResultContainer } from '../../../shared/src/components/ResultContainer'
import * as GQL from '../../../shared/src/graphql/schema'
import { SearchResultMatch } from './SearchResultMatch'

export interface HighlightRange {
    /**
     * The 0-based line number that this highlight appears in
     */
    line: number
    /**
     * The 0-based character offset to start highlighting at
     */
    character: number
    /**
     * The number of characters to highlight
     */
    length: number
}

interface Props {
    result: GQL.GenericSearchResultInterface
    isLightTheme: boolean
}

export class SearchResult extends React.Component<Props> {
    constructor(props: Props) {
        super(props)
    }

    private renderTitle = () => (
        <div className="search-result__title">
            <span
                dangerouslySetInnerHTML={{
                    __html: this.props.result.label.html
                        ? decode(this.props.result.label.html)
                        : marked(this.props.result.label.text, { gfm: true, breaks: true, sanitize: true }),
                }}
            />
            {this.props.result.detail && (
                <>
                    <span className="search-result__spacer" />
                    <small
                        dangerouslySetInnerHTML={{
                            __html: this.props.result.detail.html
                                ? decode(this.props.result.detail.html)
                                : marked(this.props.result.detail.text, { gfm: true, breaks: true, sanitize: true }),
                        }}
                    />
                </>
            )}
        </div>
    )

    private renderBody = () => (
        <>
            {this.props.result.matches.map((match, index) => {
                const highlightRanges: HighlightRange[] = []
                match.highlights.map(highlight => {
                    highlightRanges.push({
                        line: highlight.start.line,
                        character: highlight.start.character,
                        length: highlight.end.character - highlight.start.character,
                    })
                })

                return (
                    <SearchResultMatch
                        key={`item.url#${index}`}
                        item={match}
                        highlightRanges={highlightRanges}
                        isLightTheme={this.props.isLightTheme}
                    />
                )
            })}
        </>
    )

    public render(): JSX.Element {
        return (
            <ResultContainer
                stringIcon={this.props.result.icon}
                icon={FileIcon}
                collapsible={this.props.result && this.props.result.matches.length > 0}
                defaultExpanded={true}
                title={this.renderTitle()}
                expandedChildren={this.renderBody()}
            />
        )
    }
}
