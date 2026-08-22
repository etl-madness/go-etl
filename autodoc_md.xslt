<?xml version="1.0" encoding="UTF-8"?>
<xsl:stylesheet version="3.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
    <!-- Text output for Markdown -->
    <xsl:output method="text" encoding="UTF-8" omit-xml-declaration="yes" />

    <!-- Declare parameters received from external caller (e.g. Go) -->
    <xsl:param name="diagram" select="''" />
    <xsl:param name="file" select="''" />

    <!-- Newline variable for clean formatting -->
    <xsl:variable name="nl" select="'&#10;'" />

    <xsl:template match="/pipeline">
        <xsl:text># Pipeline Specification &amp; Flowchart </xsl:text><xsl:value-of select="$file" />
        <xsl:value-of select="$nl" />
        <xsl:value-of select="$nl" />

        <!-- 1. MERMAID DIAGRAM BLOCK -->
        <xsl:text>## Execution Flow Diagram</xsl:text><xsl:value-of select="$nl" />
        <xsl:value-of select="$nl" />
        <xsl:text>```mermaid</xsl:text><xsl:value-of select="$nl" />
        <xsl:value-of select="$diagram" />
        <xsl:if test="not(ends-with($diagram,$nl))">
            <xsl:value-of select="$nl" />
        </xsl:if>
        <xsl:text>```</xsl:text><xsl:value-of select="$nl" />
        <xsl:value-of select="$nl" />

        <!-- 2. PIPELINE VARIABLES -->
        <xsl:if test="variables/variable">
            <xsl:text>## Configured Variables</xsl:text><xsl:value-of select="$nl" />
            <xsl:value-of select="$nl" />
            <xsl:text>| Name | Type | Default Value |</xsl:text><xsl:value-of select="$nl" />
            <xsl:text>|---|---|---|</xsl:text><xsl:value-of select="$nl" />
            <xsl:for-each select="variables/variable">
                <xsl:text>| **</xsl:text><xsl:value-of select="@name"/><xsl:text>** | `</xsl:text>
                <xsl:value-of select="@type"/><xsl:text>` | `</xsl:text>
                <xsl:value-of select="@value"/><xsl:text>` |</xsl:text><xsl:value-of select="$nl" />
            </xsl:for-each>
            <xsl:value-of select="$nl" />
        </xsl:if>

        <!-- 3. DATABASES -->
        <xsl:if test="databases/database">
            <xsl:text>## Configured Databases</xsl:text><xsl:value-of select="$nl" />
            <xsl:value-of select="$nl" />
            <xsl:text>| Alias Name | Connection String / Variable Reference |</xsl:text><xsl:value-of select="$nl" />
            <xsl:text>|---|---|</xsl:text><xsl:value-of select="$nl" />
            <xsl:for-each select="databases/database">
                <xsl:text>| **</xsl:text><xsl:value-of select="@name"/><xsl:text>** | `</xsl:text>
                <xsl:value-of select="@connection_string"/><xsl:text>` |</xsl:text><xsl:value-of select="$nl" />
            </xsl:for-each>
            <xsl:value-of select="$nl" />
        </xsl:if>

        <!-- 4. SCRIPTS -->
        <xsl:text>## SCRIPTS</xsl:text><xsl:value-of select="$nl" />
        <xsl:value-of select="$nl" />
        <xsl:text>| Language | ID/Name | XPath Location | Source Database | Target Database | Target Table | Batch Size | Value |</xsl:text><xsl:value-of select="$nl" />
        <xsl:text>|---|---|---|---|---|---|---|---|</xsl:text><xsl:value-of select="$nl" />
        <xsl:for-each select="//script">
            <xsl:variable name="targetDb">
                <xsl:choose>
                    <xsl:when test="@target_db and normalize-space(@target_db) != ''">
                        <xsl:value-of select="@target_db"/>
                    </xsl:when>
                    <xsl:otherwise>
                        <xsl:value-of select="@db"/>
                    </xsl:otherwise>
                </xsl:choose>
            </xsl:variable>

            <!-- Clean script value:
                 1. Escape table pipe character '|' -> '&#124;'
                 2. Trim leading & trailing whitespace
                 3. Collapse consecutive line breaks and surrounding spaces into a single <br/>
            -->
            <xsl:variable name="rawVal" select="." />
            <xsl:variable name="escapedPipes" select="replace($rawVal, '\|', '&amp;#124;')" />
            <xsl:variable name="trimmedVal" select="replace($escapedPipes, '^\s+|\s+$', '')" />
            <xsl:variable name="cleanVal" select="replace($trimmedVal, '\s*[\r\n]+\s*', '&lt;br/&gt;')" />

            <xsl:text>| **</xsl:text><xsl:value-of select="@language"/><xsl:text>** | **</xsl:text>
            <xsl:value-of select="@id"/><xsl:text>** | `</xsl:text>
            <xsl:value-of select="path()"/><xsl:text>` | `</xsl:text>
            <xsl:value-of select="@db"/><xsl:text>` | `</xsl:text>
            <xsl:value-of select="normalize-space($targetDb)"/><xsl:text>` | `</xsl:text>
            <xsl:value-of select="@target_table"/><xsl:text>` | `</xsl:text>
            <xsl:value-of select="@batch_size"/><xsl:text>` | &lt;code&gt;</xsl:text>
            <xsl:value-of select="$cleanVal"/>
            <xsl:text>&lt;/code&gt; |</xsl:text>
            <xsl:value-of select="$nl" />
        </xsl:for-each>
        <xsl:value-of select="$nl" />
    </xsl:template>

    <!-- ENTRY & EXIT ID RESOLVERS -->
    <xsl:template match="script" mode="get-entry-id">
        <xsl:text>script_</xsl:text><xsl:value-of select="if (@id) then @id else generate-id()"/>
    </xsl:template>
    <xsl:template match="parallel" mode="get-entry-id">
        <xsl:text>par_start_</xsl:text><xsl:value-of select="if (@id) then @id else generate-id()"/>
    </xsl:template>
    <xsl:template match="child::if" mode="get-entry-id">
        <xsl:text>if_start_</xsl:text><xsl:value-of select="if (@id) then @id else generate-id()"/>
    </xsl:template>
    <xsl:template match="foreach|loop|while" mode="get-entry-id">
        <xsl:text>loop_start_</xsl:text><xsl:value-of select="if (@id) then @id else generate-id()"/>
    </xsl:template>
    <xsl:template match="group" mode="get-entry-id">
        <xsl:choose>
            <xsl:when test="*"><xsl:apply-templates select="*[1]" mode="get-entry-id"/></xsl:when>
            <xsl:otherwise><xsl:text>grp_</xsl:text><xsl:value-of select="if (@id) then @id else generate-id()"/></xsl:otherwise>
        </xsl:choose>
    </xsl:template>

    <xsl:template match="script" mode="get-exit-id">
        <xsl:text>script_</xsl:text><xsl:value-of select="if (@id) then @id else generate-id()"/>
    </xsl:template>
    <xsl:template match="parallel" mode="get-exit-id">
        <xsl:text>par_end_</xsl:text><xsl:value-of select="if (@id) then @id else generate-id()"/>
    </xsl:template>
    <xsl:template match="child::if" mode="get-exit-id">
        <xsl:text>if_end_</xsl:text><xsl:value-of select="if (@id) then @id else generate-id()"/>
    </xsl:template>
    <xsl:template match="foreach|loop|while" mode="get-exit-id">
        <xsl:text>loop_end_</xsl:text><xsl:value-of select="if (@id) then @id else generate-id()"/>
    </xsl:template>
    <xsl:template match="group" mode="get-exit-id">
        <xsl:choose>
            <xsl:when test="*"><xsl:apply-templates select="*[last()]" mode="get-exit-id"/></xsl:when>
            <xsl:otherwise><xsl:text>grp_</xsl:text><xsl:value-of select="if (@id) then @id else generate-id()"/></xsl:otherwise>
        </xsl:choose>
    </xsl:template>

    <!-- MERMAID GENERATOR TEMPLATES -->
    <xsl:template match="script" mode="render-mermaid">
        <xsl:variable name="nodeId" select="concat('script_', if (@id) then @id else generate-id())" />
        <xsl:variable name="scriptId">
            <xsl:choose>
                <xsl:when test="@id"><xsl:value-of select="@id"/></xsl:when>
                <xsl:otherwise>script_<xsl:value-of select="generate-id()"/></xsl:otherwise>
            </xsl:choose>
        </xsl:variable>
        <xsl:variable name="lang">
            <xsl:choose>
                <xsl:when test="@language"><xsl:value-of select="@language"/></xsl:when>
                <xsl:when test="@lang"><xsl:value-of select="@lang"/></xsl:when>
                <xsl:otherwise>SQL</xsl:otherwise>
            </xsl:choose>
        </xsl:variable>

    <xsl:value-of select="$nodeId"/>["<xsl:value-of select="$scriptId"/> &lt;br/&gt; (<xsl:value-of select="upper-case($lang)"/>)<xsl:if test="@target_db"> &lt;br/&gt; ➔ Stream to <xsl:value-of select="@target_db"/></xsl:if>"]
        <xsl:if test="following-sibling::*">
            <xsl:variable name="nextEntry"><xsl:apply-templates select="following-sibling::*[1]" mode="get-entry-id"/></xsl:variable>
    <xsl:value-of select="$nodeId"/> --&gt; <xsl:value-of select="$nextEntry"/>
        </xsl:if>
    </xsl:template>

    <xsl:template match="parallel" mode="render-mermaid">
        <xsl:variable name="pStart" select="concat('par_start_', if (@id) then @id else generate-id())"/>
        <xsl:variable name="pEnd" select="concat('par_end_', if (@id) then @id else generate-id())"/>

    <xsl:value-of select="$pStart"/>{"⚡ Parallel Execution"}
    <xsl:value-of select="$pEnd"/>(( Join ))
        <xsl:apply-templates select="*" mode="render-mermaid"/>
        <xsl:for-each select="*">
            <xsl:variable name="cEntry"><xsl:apply-templates select="." mode="get-entry-id"/></xsl:variable>
            <xsl:variable name="cExit"><xsl:apply-templates select="." mode="get-exit-id"/></xsl:variable>
    <xsl:value-of select="$pStart"/> --&gt; <xsl:value-of select="$cEntry"/>
    <xsl:value-of select="$cExit"/> --&gt; <xsl:value-of select="$pEnd"/>
        </xsl:for-each>
        <xsl:if test="following-sibling::*">
            <xsl:variable name="nextEntry"><xsl:apply-templates select="following-sibling::*[1]" mode="get-entry-id"/></xsl:variable>
    <xsl:value-of select="$pEnd"/> --&gt; <xsl:value-of select="$nextEntry"/>
        </xsl:if>
    </xsl:template>

    <xsl:template match="child::if" mode="render-mermaid">
        <xsl:variable name="ifStart" select="concat('if_start_', if (@id) then @id else generate-id())"/>
        <xsl:variable name="ifEnd" select="concat('if_end_', if (@id) then @id else generate-id())"/>
        <xsl:variable name="cond">
            <xsl:choose>
                <xsl:when test="@condition"><xsl:value-of select="@condition"/></xsl:when>
                <xsl:when test="@var"><xsl:value-of select="@var"/> == <xsl:value-of select="@equals"/></xsl:when>
                <xsl:otherwise>Check Condition</xsl:otherwise>
            </xsl:choose>
        </xsl:variable>

    <xsl:value-of select="$ifStart"/>{"❓ If: <xsl:value-of select="replace(translate($cond, '&quot;', &quot;'&quot;), '&quot;', &quot;'&quot;)"/>"}
    <xsl:value-of select="$ifEnd"/>(( Rejoin ))

        <xsl:choose>
            <xsl:when test="then/*">
                <xsl:apply-templates select="then/*" mode="render-mermaid"/>
                <xsl:variable name="thenEntry"><xsl:apply-templates select="then/*[1]" mode="get-entry-id"/></xsl:variable>
                <xsl:variable name="thenExit"><xsl:apply-templates select="then/*[last()]" mode="get-exit-id"/></xsl:variable>
    <xsl:value-of select="$ifStart"/> -- "Yes / Then" --&gt; <xsl:value-of select="$thenEntry"/>
    <xsl:value-of select="$thenExit"/> --&gt; <xsl:value-of select="$ifEnd"/>
            </xsl:when>
            <xsl:when test="*[not(self::then or self::else)]">
                <xsl:apply-templates select="*[not(self::then or self::else)]" mode="render-mermaid"/>
                <xsl:variable name="thenEntry"><xsl:apply-templates select="*[not(self::then or self::else)][1]" mode="get-entry-id"/></xsl:variable>
                <xsl:variable name="thenExit"><xsl:apply-templates select="*[not(self::then or self::else)][last()]" mode="get-exit-id"/></xsl:variable>
    <xsl:value-of select="$ifStart"/> -- "Yes / Then" --&gt; <xsl:value-of select="$thenEntry"/>
    <xsl:value-of select="$thenExit"/> --&gt; <xsl:value-of select="$ifEnd"/>
            </xsl:when>
            <xsl:otherwise>
    <xsl:value-of select="$ifStart"/> -- "Yes / Then" --&gt; <xsl:value-of select="$ifEnd"/>
            </xsl:otherwise>
        </xsl:choose>

        <xsl:choose>
            <xsl:when test="else/*">
                <xsl:apply-templates select="else/*" mode="render-mermaid"/>
                <xsl:variable name="elseEntry"><xsl:apply-templates select="else/*[1]" mode="get-entry-id"/></xsl:variable>
                <xsl:variable name="elseExit"><xsl:apply-templates select="else/*[last()]" mode="get-exit-id"/></xsl:variable>
    <xsl:value-of select="$ifStart"/> -- "No / Else" --&gt; <xsl:value-of select="$elseEntry"/>
    <xsl:value-of select="$elseExit"/> --&gt; <xsl:value-of select="$ifEnd"/>
            </xsl:when>
            <xsl:otherwise>
    <xsl:value-of select="$ifStart"/> -- "No / Else" --&gt; <xsl:value-of select="$elseEntry"/>
            </xsl:otherwise>
        </xsl:choose>

        <xsl:if test="following-sibling::*">
            <xsl:variable name="nextEntry"><xsl:apply-templates select="following-sibling::*[1]" mode="get-entry-id"/></xsl:variable>
    <xsl:value-of select="$ifEnd"/> --&gt; <xsl:value-of select="$nextEntry"/>
        </xsl:if>
    </xsl:template>

    <xsl:template match="foreach|loop|while" mode="render-mermaid">
        <xsl:variable name="loopStart" select="concat('loop_start_', if (@id) then @id else generate-id())"/>
        <xsl:variable name="loopEnd" select="concat('loop_end_', if (@id) then @id else generate-id())"/>
        <xsl:variable name="loopName">
            <xsl:choose>
                <xsl:when test="@id"><xsl:value-of select="@id"/></xsl:when>
                <xsl:otherwise>Loop</xsl:otherwise>
            </xsl:choose>
        </xsl:variable>

    <xsl:value-of select="$loopStart"/>{"🔄 Loop: <xsl:value-of select="$loopName"/><xsl:if test="@var"> (<xsl:value-of select="@var"/>)</xsl:if>"}
    <xsl:value-of select="$loopEnd"/>(( Loop Exit ))

        <xsl:if test="*">
            <xsl:apply-templates select="*" mode="render-mermaid"/>
            <xsl:variable name="cEntry"><xsl:apply-templates select="*[1]" mode="get-entry-id"/></xsl:variable>
            <xsl:variable name="cExit"><xsl:apply-templates select="*[last()]" mode="get-exit-id"/></xsl:variable>
    <xsl:value-of select="$loopStart"/> -- "Next Row" --&gt; <xsl:value-of select="$cEntry"/>
    <xsl:value-of select="$cExit"/> --&gt; <xsl:value-of select="$loopStart"/>
        </xsl:if>
    <xsl:value-of select="$loopStart"/> -- "Done" --&gt; <xsl:value-of select="$loopEnd"/>

        <xsl:if test="following-sibling::*">
            <xsl:variable name="nextEntry"><xsl:apply-templates select="following-sibling::*[1]" mode="get-entry-id"/></xsl:variable>
    <xsl:value-of select="$loopEnd"/> --&gt; <xsl:value-of select="$nextEntry"/>
        </xsl:if>
    </xsl:template>

    <xsl:template match="group" mode="render-mermaid">
        <xsl:apply-templates select="*" mode="render-mermaid"/>
        <xsl:if test="following-sibling::*">
            <xsl:variable name="groupExit"><xsl:apply-templates select="." mode="get-exit-id"/></xsl:variable>
            <xsl:variable name="nextEntry"><xsl:apply-templates select="following-sibling::*[1]" mode="get-entry-id"/></xsl:variable>
    <xsl:value-of select="$groupExit"/> --&gt; <xsl:value-of select="$nextEntry"/>
        </xsl:if>
    </xsl:template>

</xsl:stylesheet>